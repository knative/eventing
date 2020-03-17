/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package fanout provides an http.Handler that takes in one request and fans it out to N other
// requests, based on a list of Subscriptions. Logically, it represents all the Subscriptions to a
// single Knative Channel.
// It will normally be used in conjunction with multichannelfanout.Handler, which contains multiple
// fanout.Handlers, each corresponding to a single Knative Channel.
package fanout

import (
	"context"
	"errors"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel"
)

const (
	defaultTimeout = 15 * time.Minute

	eventBufferSize = 500
)

// Config for a fanout.Handler.
type Config struct {
	Subscriptions []eventingduck.SubscriberSpec `json:"subscriptions"`
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	AsyncHandler     bool `json:"asyncHandler,omitempty"`
	DispatcherConfig channel.EventDispatcherConfig
}

// Handler is a http.Handler that takes a single request in and fans it out to N other servers.
type Handler struct {
	config Config

	receivedEvents chan *forwardEvent
	receiver       *channel.EventReceiver
	dispatcher     *channel.EventDispatcher

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	logger *zap.Logger
}

// forwardEvent is passed between the Receiver and the Dispatcher.
type forwardEvent struct {
	event cloudevents.Event
	done  chan<- error
}

// NewHandler creates a new fanout.Handler.
func NewHandler(logger *zap.Logger, config Config) (*Handler, error) {
	handler := &Handler{
		logger:         logger,
		config:         config,
		dispatcher:     channel.NewEventDispatcherFromConfig(logger, config.DispatcherConfig),
		receivedEvents: make(chan *forwardEvent, eventBufferSize),
		timeout:        defaultTimeout,
	}
	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	receiver, err := channel.NewEventReceiver(createReceiverFunction(handler), logger)
	if err != nil {
		return nil, err
	}
	handler.receiver = receiver
	return handler, nil
}

func createReceiverFunction(f *Handler) func(context.Context, channel.ChannelReference, cloudevents.Event) error {
	return func(ctx context.Context, _ channel.ChannelReference, event cloudevents.Event) error {
		if f.config.AsyncHandler {
			parentSpan := trace.FromContext(ctx)
			go func() {
				// Run async dispatch with background context.
				ctx = trace.NewContext(context.Background(), parentSpan)
				// Any returned error is already logged in f.dispatch().
				_ = f.dispatch(ctx, event)
			}()
			return nil
		}
		return f.dispatch(ctx, event)
	}
}

func (f *Handler) ServeHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	return f.receiver.ServeHTTP(ctx, event, resp)
}

// dispatch takes the event, fans it out to each subscription in f.config. If all the fanned out
// events return successfully, then return nil. Else, return an error.
func (f *Handler) dispatch(ctx context.Context, event cloudevents.Event) error {
	errorCh := make(chan error, len(f.config.Subscriptions))
	for _, sub := range f.config.Subscriptions {
		go func(s eventingduck.SubscriberSpec) {
			errorCh <- f.makeFanoutRequest(ctx, event, s)
		}(sub)
	}

	for range f.config.Subscriptions {
		select {
		case err := <-errorCh:
			if err != nil {
				f.logger.Error("Fanout had an error", zap.Error(err))
				return err
			}
		case <-time.After(f.timeout):
			f.logger.Error("Fanout timed out")
			return errors.New("fanout timed out")
		}
	}
	// All Subscriptions returned err = nil.
	return nil
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (f *Handler) makeFanoutRequest(ctx context.Context, event cloudevents.Event, sub eventingduck.SubscriberSpec) error {
	return f.dispatcher.DispatchEventWithDelivery(ctx, event, sub.SubscriberURI.String(), sub.ReplyURI.String(), &channel.DeliveryOptions{DeadLetterSink: sub.DeadLetterSinkURI.String()})
}
