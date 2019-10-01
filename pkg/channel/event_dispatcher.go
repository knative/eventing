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

	"knative.dev/pkg/apis"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const correlationIDHeaderName = "Knative-Correlation-Id"

type Dispatcher interface {
	// DispatchEvent dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply *apis.URL) error
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
	ceClient, err := kncloudevents.NewDefaultClient()
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
func (d *EventDispatcher) DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply *apis.URL) error {
	var err error
	// Default to replying with the original event. If there is a destination, then replace it
	// with the response from the call to the destination instead.
	response := &event
	if destination != nil {

		ctx, response, err = d.executeRequest(ctx, destination.URL(), event)
		if err != nil {
			return fmt.Errorf("unable to complete request to %s: %v", destination.String(), err)
		}
	}

	if response == nil {
		d.logger.Debug("cannot forward response as response object is nil")
		return nil
	}

	if reply == nil {
		d.logger.Debug("cannot forward response as reply is empty", zap.Any("response", response))
		return nil
	}

	_, _, err = d.executeRequest(ctx, reply.URL(), *response)
	if err != nil {
		return fmt.Errorf("failed to forward reply to %s: %v", reply.String(), err)
	}
	return nil
}

func (d *EventDispatcher) executeRequest(ctx context.Context, url *url.URL, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	d.logger.Debug("Dispatching event", zap.String("event.id", event.ID()), zap.String("url", url.String()))
	originalTransportCTX := cloudevents.HTTPTransportContextFrom(ctx)
	sendingCTX := d.generateSendingContext(originalTransportCTX, url, event)

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

func (d *EventDispatcher) generateSendingContext(originalTransportCTX cehttp.TransportContext, url *url.URL, event cloudevents.Event) context.Context {
	sctx := utils.ContextFrom(originalTransportCTX, url)
	sctx, err := tracing.AddSpanFromTraceparentAttribute(sctx, url.Path, event)
	if err != nil {
		d.logger.Info("Unable to connect outgoing span", zap.Error(err))
	}
	return sctx
}

func generateReplyContext(rctx context.Context, originalTransportCTX cehttp.TransportContext) (context.Context, error) {
	// rtctx = Reply transport context
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	if isFailure(rtctx.StatusCode) {
		// Reject non-successful responses.
		return rctx, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", rtctx.StatusCode)
	}
	headers := utils.PassThroughHeaders(rtctx.Header)
	if correlationID, ok := originalTransportCTX.Header[correlationIDHeaderName]; ok {
		headers[correlationIDHeaderName] = correlationID
	}
	rtctx.Header = http.Header(headers)
	rctx = cehttp.WithTransportContext(rctx, rtctx)
	return rctx, nil
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < http.StatusOK /* 200 */ ||
		statusCode >= http.StatusMultipleChoices /* 300 */
}
