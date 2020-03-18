/*
Copyright 2020 The Knative Authors

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
	nethttp "net/http"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel"
)

// Handler is a http.Handler that takes a single request in and fans it out to N other servers.
type MessageHandler struct {
	config Config

	receiver   *channel.MessageReceiver
	dispatcher *channel.MessageDispatcherImpl

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	logger *zap.Logger
}

// NewHandler creates a new fanout.Handler.
func NewMessageHandler(logger *zap.Logger, config Config) (*MessageHandler, error) {
	handler := &MessageHandler{
		logger:     logger,
		config:     config,
		dispatcher: channel.NewMessageDispatcherFromConfig(logger, config.DispatcherConfig),
		timeout:    defaultTimeout,
	}
	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	receiver, err := channel.NewMessageReceiver(createMessageReceiverFunction(handler), logger)
	if err != nil {
		return nil, err
	}
	handler.receiver = receiver
	return handler, nil
}

func createMessageReceiverFunction(f *MessageHandler) func(context.Context, channel.ChannelReference, binding.Message, []binding.TransformerFactory, nethttp.Header) error {
	if f.config.AsyncHandler {
		return func(ctx context.Context, _ channel.ChannelReference, message binding.Message, transformers []binding.TransformerFactory, additionalHeaders nethttp.Header) error {
			parentSpan := trace.FromContext(ctx)
			go func() {
				// Run async dispatch with background context.
				ctx = trace.NewContext(context.Background(), parentSpan)
				// Any returned error is already logged in f.dispatch().
				_ = f.dispatch(ctx, message, transformers, additionalHeaders)
			}()
			return nil
		}
	}
	return func(ctx context.Context, _ channel.ChannelReference, message binding.Message, transformers []binding.TransformerFactory, additionalHeaders nethttp.Header) error {
		return f.dispatch(ctx, message, transformers, additionalHeaders)
	}
}

func (f *MessageHandler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	f.receiver.ServeHTTP(response, request)
}

// dispatch takes the event, fans it out to each subscription in f.config. If all the fanned out
// events return successfully, then return nil. Else, return an error.
func (f *MessageHandler) dispatch(ctx context.Context, originalMessage binding.Message, transformers []binding.TransformerFactory, additionalHeaders nethttp.Header) error {
	subs := len(f.config.Subscriptions)

	// We buffer the message to send it several times
	bufferedMessage, err := buffering.CopyMessage(ctx, originalMessage, transformers)
	if err != nil {
		return err
	}
	// We don't need the original message anymore
	_ = originalMessage.Finish(nil)

	// Bind the lifecycle of the buffered message to the number of subs
	bufferedMessage = buffering.WithAcksBeforeFinish(bufferedMessage, subs)

	errorCh := make(chan error, subs)
	for _, sub := range f.config.Subscriptions {
		go func(s eventingduck.SubscriberSpec) {
			errorCh <- f.makeFanoutRequest(ctx, bufferedMessage, additionalHeaders, s)
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
func (f *MessageHandler) makeFanoutRequest(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, sub eventingduck.SubscriberSpec) error {
	return f.dispatcher.DispatchMessageWithDelivery(ctx, message, additionalHeaders, sub.SubscriberURI.String(), sub.ReplyURI.String(), &channel.DeliveryOptions{DeadLetterSink: sub.DeadLetterSinkURI.String()})
}
