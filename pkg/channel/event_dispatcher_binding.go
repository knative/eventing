/*
 * Copyright 2020 The Knative Authors
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
	nethttp "net/http"
	"net/url"

	"github.com/cloudevents/sdk-go/pkg/binding"
	"github.com/cloudevents/sdk-go/pkg/protocol/http"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
)

type DispatcherBinding interface {
	// DispatchEvent dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchEvent(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, destination, reply string) error

	// DispatchEventWithDelivery dispatches an event to a destination over HTTP with delivery options
	//
	// The destination and reply are URLs.
	DispatchEventWithDelivery(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, destination, reply string, delivery *DeliveryOptions) error
}

// EventDispatcherBinding is the 'real' DispatcherBinding used everywhere except unit tests.
var _ DispatcherBinding = &EventDispatcherBinding{}

// EventDispatcherBinding dispatches events to a destination over HTTP.
type EventDispatcherBinding struct {
	sender           *kncloudevents.HttpBindingSender
	supportedSchemes sets.String

	logger *zap.Logger
}

// NewEventDispatcherBinding creates a new event dispatcher that can dispatch
// events to HTTP destinations.
func NewEventDispatcherBinding(logger *zap.Logger) *EventDispatcherBinding {
	return NewEventDispatcherBindingFromConfig(logger, defaultEventDispatcherConfig)
}

// NewEventDispatcherBindingFromConfig creates a new event dispatcher based on config.
func NewEventDispatcherBindingFromConfig(logger *zap.Logger, config EventDispatcherConfig) *EventDispatcherBinding {
	sender, err := kncloudevents.NewHttpBindingsSender(&config.ConnectionArgs, "")
	if err != nil {
		logger.Fatal("Unable to create cloudevents binding sender", zap.Error(err))
		return nil
	}
	return &EventDispatcherBinding{
		sender:           sender,
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

// DispatchEvent dispatches an event to a destination over HTTP.
//
// The destination and reply are URLs.
func (d *EventDispatcherBinding) DispatchEvent(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, destination, reply string) error {
	return d.DispatchEventWithDelivery(ctx, message, additionalHeaders, destination, reply, nil)
}

// DispatchEventWithDelivery dispatches an event to a destination over HTTP with delivery options
//
// The destination and reply are URLs.
func (d *EventDispatcherBinding) DispatchEventWithDelivery(ctx context.Context, message binding.Message, initialAdditionalHeaders nethttp.Header, destination, reply string, delivery *DeliveryOptions) error {
	var err error

	// Default to replying with the original event. If there is a destination, then replace it
	// with the response from the call to the destination instead.
	response := message
	additionalHeaders := initialAdditionalHeaders
	if destination != "" {
		defer message.Finish(nil)

		destinationURL := d.resolveURL(destination)
		response, additionalHeaders, err = d.executeRequest(ctx, destinationURL, message, initialAdditionalHeaders)
		if err != nil {
			if delivery != nil && delivery.DeadLetterSink != "" {
				deadLetterURL := d.resolveURL(delivery.DeadLetterSink)

				// TODO: decorate event with deadletter attributes
				_, _, err2 := d.executeRequest(ctx, deadLetterURL, message, initialAdditionalHeaders)
				if err2 != nil {
					return fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destinationURL, err, deadLetterURL, err2)
				}

				// Do not send event to reply
				return nil
			}
			return fmt.Errorf("unable to complete request to %s: %v", destinationURL, err)
		}
	}

	defer func() {
		if response != nil {
			_ = response.Finish(nil)
		}
	}()

	if reply == "" && response != nil {
		d.logger.Debug("cannot forward response as reply is empty", zap.Any("response", response))
		return nil
	}

	if reply != "" && response != nil {
		replyURL := d.resolveURL(reply)
		_, _, err = d.executeRequest(ctx, replyURL, response, additionalHeaders)
		if err != nil {
			if delivery != nil && delivery.DeadLetterSink != "" {
				deadLetterURL := d.resolveURL(delivery.DeadLetterSink)

				// TODO: decorate event with deadletter attributes
				_, _, err2 := d.executeRequest(ctx, deadLetterURL, message, additionalHeaders)
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

func (d *EventDispatcherBinding) executeRequest(ctx context.Context, url *url.URL, message binding.Message, additionalHeaders nethttp.Header) (binding.Message, nethttp.Header, error) {
	d.logger.Debug("Dispatching event", zap.String("url", url.String()))

	req, err := d.sender.NewCloudEventRequestWithTarget(ctx, url.String())
	if err != nil {
		return nil, nil, err
	}

	err = kncloudevents.WriteHttpRequest(ctx, message, req, additionalHeaders, []binding.TransformerFactory{})
	if err != nil {
		return nil, nil, err
	}

	response, err := d.sender.Send(req)
	if err != nil {
		return nil, nil, err
	}
	if isFailure(response.StatusCode) {
		// Reject non-successful responses.
		return nil, nil, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", response.StatusCode)
	}
	responseMessage := http.NewMessageFromHttpResponse(response)
	if responseMessage.ReadEncoding() == binding.EncodingUnknown {
		d.logger.Debug("Response is a non event, discarding it", zap.Int("status_code", response.StatusCode))
		return nil, nil, nil
	}
	return responseMessage, utils.PassThroughHeaders(response.Header), nil
}

func (d *EventDispatcherBinding) resolveURL(destination string) *url.URL {
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
