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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

type MessageDispatcher interface {
	// DispatchMessage dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL) error

	// DispatchMessageWithRetries dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL, config *kncloudevents.RetryConfig) error
}

// MessageDispatcherImpl is the 'real' MessageDispatcher used everywhere except unit tests.
var _ MessageDispatcher = &MessageDispatcherImpl{}

// MessageDispatcherImpl dispatches events to a destination over HTTP.
type MessageDispatcherImpl struct {
	sender           *kncloudevents.HttpMessageSender
	supportedSchemes sets.String

	logger *zap.Logger
}

// NewMessageDispatcher creates a new Message dispatcher that can dispatch
// events to HTTP destinations.
func NewMessageDispatcher(logger *zap.Logger) *MessageDispatcherImpl {
	return NewMessageDispatcherFromConfig(logger, defaultEventDispatcherConfig)
}

// NewMessageDispatcherFromConfig creates a new Message dispatcher based on config.
func NewMessageDispatcherFromConfig(logger *zap.Logger, config EventDispatcherConfig) *MessageDispatcherImpl {
	sender, err := kncloudevents.NewHttpMessageSender(&config.ConnectionArgs, "")
	if err != nil {
		logger.Fatal("Unable to create cloudevents binding sender", zap.Error(err))
	}
	return NewMessageDispatcherFromSender(logger, sender)
}

// NewMessageDispatcherFromConfig creates a new event dispatcher.
func NewMessageDispatcherFromSender(logger *zap.Logger, sender *kncloudevents.HttpMessageSender) *MessageDispatcherImpl {
	return &MessageDispatcherImpl{
		sender:           sender,
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

func (d *MessageDispatcherImpl) DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL) error {
	return d.DispatchMessageWithRetries(ctx, message, additionalHeaders, destination, reply, deadLetter, nil)
}

func (d *MessageDispatcherImpl) DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL, retriesConfig *kncloudevents.RetryConfig) error {

	// All messages that should be finished at the end of this function
	// are placed in this slice
	var messagesToFinish []binding.Message
	defer func() {
		for _, msg := range messagesToFinish {
			_ = msg.Finish(nil)
		}
	}()

	// sanitize eventual host-only URLs
	destination = d.sanitizeURL(destination)
	reply = d.sanitizeURL(reply)
	deadLetter = d.sanitizeURL(deadLetter)

	// If there is a destination, variables response* are filled with the response of the destination
	// Otherwise, they are filled with the original message
	var responseMessage cloudevents.Message
	var responseAdditionalHeaders nethttp.Header

	if destination != nil {
		var err error
		// Try to send to destination
		messagesToFinish = append(messagesToFinish, message)

		ctx, responseMessage, responseAdditionalHeaders, err = d.executeRequest(ctx, destination, message, additionalHeaders, retriesConfig)
		if err != nil {
			// DeadLetter is configured, send the message to it
			if deadLetter != nil {

				_, deadLetterResponse, _, deadLetterErr := d.executeRequest(ctx, deadLetter, message, additionalHeaders, retriesConfig)
				if deadLetterErr != nil {
					return fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destination, err, deadLetter, deadLetterErr)
				}
				if deadLetterResponse != nil {
					messagesToFinish = append(messagesToFinish, deadLetterResponse)
				}

				return nil
			}
			// No DeadLetter, just fail
			return fmt.Errorf("unable to complete request to %s: %v", destination, err)
		}
	} else {
		// No destination url, try to send to reply if available
		responseMessage = message
		responseAdditionalHeaders = additionalHeaders
	}

	// No response, dispatch completed
	if responseMessage == nil {
		return nil
	}

	messagesToFinish = append(messagesToFinish, responseMessage)

	if reply == nil {
		d.logger.Debug("cannot forward response as reply is empty")
		return nil
	}

	ctx, responseResponseMessage, _, err := d.executeRequest(ctx, reply, responseMessage, responseAdditionalHeaders, retriesConfig)
	if err != nil {
		// DeadLetter is configured, send the message to it
		if deadLetter != nil {
			_, deadLetterResponse, _, deadLetterErr := d.executeRequest(ctx, deadLetter, message, responseAdditionalHeaders, retriesConfig)
			if deadLetterErr != nil {
				return fmt.Errorf("failed to forward reply to %s (%v) and failed to send it to the dead letter sink %s (%v)", reply, err, deadLetter, deadLetterErr)
			}
			if deadLetterResponse != nil {
				messagesToFinish = append(messagesToFinish, deadLetterResponse)
			}

			return nil
		}
		// No DeadLetter, just fail
		return fmt.Errorf("failed to forward reply to %s: %v", reply, err)
	}
	if responseResponseMessage != nil {
		messagesToFinish = append(messagesToFinish, responseResponseMessage)
	}

	return nil
}

func (d *MessageDispatcherImpl) executeRequest(ctx context.Context, url *url.URL, message cloudevents.Message, additionalHeaders nethttp.Header, configs *kncloudevents.RetryConfig) (context.Context, cloudevents.Message, nethttp.Header, error) {
	d.logger.Debug("Dispatching event", zap.String("url", url.String()))

	ctx, span := trace.StartSpan(ctx, "knative.dev", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req, err := d.sender.NewCloudEventRequestWithTarget(ctx, url.String())
	if err != nil {
		return ctx, nil, nil, err
	}

	if span.IsRecordingEvents() {
		err = kncloudevents.WriteHttpRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders, tracing.PopulateSpan(span))
	} else {
		err = kncloudevents.WriteHttpRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders)
	}
	if err != nil {
		return ctx, nil, nil, err
	}

	response, err := d.sender.SendWithRetries(req, configs)
	if err != nil {
		return ctx, nil, nil, err
	}
	if isFailure(response.StatusCode) {
		_ = response.Body.Close()
		// Reject non-successful responses.
		return ctx, nil, nil, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", response.StatusCode)
	}
	responseMessage := http.NewMessageFromHttpResponse(response)
	if responseMessage.ReadEncoding() == binding.EncodingUnknown {
		_ = response.Body.Close()
		d.logger.Debug("Response is a non event, discarding it", zap.Int("status_code", response.StatusCode))
		return ctx, nil, nil, nil
	}
	return ctx, responseMessage, utils.PassThroughHeaders(response.Header), nil
}

func (d *MessageDispatcherImpl) sanitizeURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	if d.supportedSchemes.Has(u.Scheme) {
		// Already a URL with a known scheme.
		return u
	}
	return &url.URL{
		Scheme: "http",
		Host:   u.Host,
		Path:   "/",
	}
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < nethttp.StatusOK /* 200 */ ||
		statusCode >= nethttp.StatusMultipleChoices /* 300 */
}
