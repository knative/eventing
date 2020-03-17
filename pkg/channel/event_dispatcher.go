/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package channel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	cehttp "github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/transport/http"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	pkgtracing "knative.dev/pkg/tracing"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
)

const (
	correlationIDHeaderName = "Knative-Correlation-Id"

	// Defaults for the underlying HTTP Client transport. These would enable better connection reuse.
	// Set them on a 10:1 ratio, but this would actually depend on the Subscriptions' subscribers and the workload itself.
	// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
	// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 100
)

type Dispatcher interface {
	// DispatchEvent dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply string) error

	// DispatchEventWithDelivery dispatches an event to a destination over HTTP with delivery options
	//
	// The destination and reply are URLs.
	DispatchEventWithDelivery(ctx context.Context, event cloudevents.Event, destination, reply string, delivery *DeliveryOptions) error
}

// DeliveryOptions are the delivery options supported by this dispatcher
type DeliveryOptions struct {
	// DeadLetterSink is the sink receiving events that could not be dispatched.
	DeadLetterSink string
}

// EventDispatcher is the 'real' Dispatcher used everywhere except unit tests.
var _ Dispatcher = &EventDispatcher{}

var propagation = &b3.HTTPFormat{}

// EventDispatcher dispatches events to a destination over HTTP.
type EventDispatcher struct {
	ceClient         cloudevents.Client
	supportedSchemes sets.String

	logger *zap.Logger
}

// NewEventDispatcher creates a new event dispatcher that can dispatch
// events to HTTP destinations.
func NewEventDispatcher(logger *zap.Logger) *EventDispatcher {
	return NewEventDispatcherFromConfig(logger, defaultEventDispatcherConfig)
}

// NewEventDispatcherFromConfig creates a new event dispatcher based on config.
func NewEventDispatcherFromConfig(logger *zap.Logger, config EventDispatcherConfig) *EventDispatcher {
	httpTransport, err := cloudevents.NewHTTPTransport(
		cloudevents.WithBinaryEncoding(),
		cloudevents.WithMiddleware(pkgtracing.HTTPSpanMiddleware))
	if err != nil {
		logger.Fatal("Unable to create CE transport", zap.Error(err))
	}
	cArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
	}
	ceClient, err := kncloudevents.NewDefaultClientGivenHttpTransport(httpTransport, &cArgs)
	if err != nil {
		logger.Fatal("failed to create cloudevents client", zap.Error(err))
	}
	return &EventDispatcher{
		ceClient:         ceClient,
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

// DispatchEvent dispatches an event to a destination over HTTP.
//
// The destination and reply are URLs.
func (d *EventDispatcher) DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply string) error {
	return d.DispatchEventWithDelivery(ctx, event, destination, reply, nil)
}

// DispatchEventWithDelivery dispatches an event to a destination over HTTP with delivery options
//
// The destination and reply are URLs.
func (d *EventDispatcher) DispatchEventWithDelivery(ctx context.Context, event cloudevents.Event, destination, reply string, delivery *DeliveryOptions) error {
	var err error
	// Default to replying with the original event. If there is a destination, then replace it
	// with the response from the call to the destination instead.
	response := &event
	var nonerrctx context.Context = ctx
	if destination != "" {
		destinationURL := d.resolveURL(destination)

		nonerrctx, response, err = d.executeRequest(ctx, destinationURL, event)
		if err != nil {

			if delivery != nil && delivery.DeadLetterSink != "" {
				deadLetterURL := d.resolveURL(delivery.DeadLetterSink)

				// TODO: decorate event with deadletter attributes
				_, _, err2 := d.executeRequest(ctx, deadLetterURL, event)
				if err2 != nil {
					return fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destinationURL, err, deadLetterURL, err2)
				}

				// Do not send event to reply
				return nil
			}
			return fmt.Errorf("unable to complete request to %s: %v", destinationURL, err)
		}
	}

	if reply == "" && response != nil {
		d.logger.Debug("cannot forward response as reply is empty", zap.Any("response", response))
		return nil
	}

	if reply != "" && response != nil {
		replyURL := d.resolveURL(reply)
		_, _, err = d.executeRequest(nonerrctx, replyURL, *response)
		if err != nil {
			if delivery != nil && delivery.DeadLetterSink != "" {
				deadLetterURL := d.resolveURL(delivery.DeadLetterSink)

				// TODO: decorate event with deadletter attributes
				_, _, err2 := d.executeRequest(nonerrctx, deadLetterURL, event)
				if err2 != nil {
					return fmt.Errorf("failed to forward reply to %s (%v) and failed to send it to the dead letter sink %s (%v)", replyURL, err, deadLetterURL, err2)
				}
			} else {
				return fmt.Errorf("failed to forward reply to %s: %v", replyURL, err)
			}
		}
	}
	return nil
}

func (d *EventDispatcher) executeRequest(ctx context.Context, url *url.URL, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	d.logger.Debug("Dispatching event", zap.String("event.id", event.ID()), zap.String("url", url.String()))
	originalTransportCTX := cloudevents.HTTPTransportContextFrom(ctx)
	sendingCTX := utils.SendingContextFrom(ctx, originalTransportCTX, url)

	replyCTX, reply, err := d.ceClient.Send(sendingCTX, event)
	if err != nil {
		return nil, nil, err
	}
	replyCTX, err = generateReplyContext(replyCTX, originalTransportCTX)
	if err != nil {
		return nil, nil, err
	}
	return replyCTX, reply, nil
}

func generateReplyContext(rctx context.Context, originalTransportCTX cehttp.TransportContext) (context.Context, error) {
	// rtctx = Reply transport context
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	if isFailure(rtctx.StatusCode) {
		// Reject non-successful responses.
		return rctx, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", rtctx.StatusCode)
	}
	rctx = utils.SendingContextFrom(rctx, rtctx, nil)
	if correlationID, ok := originalTransportCTX.Header[correlationIDHeaderName]; ok {
		for _, v := range correlationID {
			rctx = cloudevents.ContextWithHeader(rctx, correlationIDHeaderName, v)
		}
	}
	return rctx, nil
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < http.StatusOK /* 200 */ ||
		statusCode >= http.StatusMultipleChoices /* 300 */
}

func (d *EventDispatcher) resolveURL(destination string) *url.URL {
	if u, err := url.Parse(destination); err == nil && d.supportedSchemes.Has(u.Scheme) {
		// Already a URL with a known scheme.
		return u
	}
	return &url.URL{
		Scheme: "http",
		Host:   destination,
		Path:   "/",
	}
}
