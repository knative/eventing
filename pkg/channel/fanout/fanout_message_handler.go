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
// It will normally be used in conjunction with multichannelfanout.MessageHandler, which contains multiple
// fanout.MessageHandler, each corresponding to a single Knative Channel.
package fanout

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/url"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	"github.com/cloudevents/sdk-go/v2/types"
	"go.opencensus.io/trace"
	"go.uber.org/zap"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/kncloudevents"
)

const (
	defaultTimeout = 15 * time.Minute
)

type Subscription struct {
	Subscriber  *url.URL
	Reply       *url.URL
	DeadLetter  *url.URL
	RetryConfig *kncloudevents.RetryConfig
}

// Config for a fanout.MessageHandler.
type Config struct {
	Subscriptions []Subscription `json:"subscriptions"`
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	AsyncHandler bool `json:"asyncHandler,omitempty"`
}

// MessageHandler is a http.Handler that takes a single request in and fans it out to N other servers.
type MessageHandler struct {
	config Config

	receiver   *channel.MessageReceiver
	dispatcher channel.MessageDispatcher

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	reporter channel.StatsReporter
	logger *zap.Logger
}

// NewMessageHandler creates a new fanout.MessageHandler.
func NewMessageHandler(logger *zap.Logger, messageDispatcher channel.MessageDispatcher, config Config, reporter channel.StatsReporter) (*MessageHandler, error) {
	handler := &MessageHandler{
		logger:     logger,
		config:     config,
		dispatcher: messageDispatcher,
		timeout:    defaultTimeout,
		reporter: reporter,
	}
	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	receiver, err := channel.NewMessageReceiver(createMessageReceiverFunction(handler), logger, reporter)
	if err != nil {
		return nil, err
	}
	handler.receiver = receiver

	return handler, nil
}

func SubscriberSpecToFanoutConfig(sub eventingduckv1.SubscriberSpec) (*Subscription, error) {
	var destination *url.URL
	if sub.SubscriberURI != nil {
		destination = sub.SubscriberURI.URL()
	}

	var reply *url.URL
	if sub.ReplyURI != nil {
		reply = sub.ReplyURI.URL()
	}

	var deadLetter *url.URL
	if sub.Delivery != nil && sub.Delivery.DeadLetterSink != nil && sub.Delivery.DeadLetterSink.URI != nil {
		deadLetter = sub.Delivery.DeadLetterSink.URI.URL()
	}

	var retryConfig *kncloudevents.RetryConfig
	if sub.Delivery != nil {
		if rc, err := kncloudevents.RetryConfigFromDeliverySpec(*sub.Delivery); err != nil {
			return nil, err
		} else {
			retryConfig = &rc
		}
	}

	return &Subscription{Subscriber: destination, Reply: reply, DeadLetter: deadLetter, RetryConfig: retryConfig}, nil
}

func createMessageReceiverFunction(f *MessageHandler) func(context.Context, channel.ChannelReference, binding.Message, []binding.Transformer, nethttp.Header) error {
	if f.config.AsyncHandler {
		return func(ctx context.Context, ref channel.ChannelReference, message binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
			if len(f.config.Subscriptions) == 0 {
				// Nothing to do here, finish the message and return
				_ = message.Finish(nil)
				return nil
			}

			parentSpan := trace.FromContext(ctx)
			// Message buffering here is done before starting the dispatch goroutine
			// Because the message could be closed before the buffering happens
			bufferedMessage, err := buffering.CopyMessage(ctx, message, transformers...)
			if err != nil {
				return err
			}

			var te channel.TypeExtractorTransformer
			for _, tr := range transformers {
				switch tr.(type) {
				case channel.TypeExtractorTransformer:
					te = tr.(channel.TypeExtractorTransformer)
				}
			}

			reportArgs := channel.ReportArgs{}
			eventType, err := types.ToString(te)
			if err != nil{
				reportArgs.EventType = eventType
			}
			reportArgs.Ns = ref.Namespace

			// We don't need the original message anymore
			_ = message.Finish(nil)
			go func(m binding.Message, h nethttp.Header, s *trace.Span, r *channel.StatsReporter, args *channel.ReportArgs) {
				// Run async dispatch with background context.
				ctx = trace.NewContext(context.Background(), s)
				// Any returned error is already logged in f.dispatch().
				totalDispatchTime, _ := f.dispatch(ctx, m, h)
				if totalDispatchTime > channel.NoDuration {
					_ = (*r).ReportEventDispatchTime(args, 0, totalDispatchTime)
				}
			}(bufferedMessage, additionalHeaders, parentSpan, &f.reporter, &reportArgs)
			return nil
		}
	}
	return func(ctx context.Context, ref channel.ChannelReference, message binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
		if len(f.config.Subscriptions) == 0 {
			// Nothing to do here, finish the message and return
			_ = message.Finish(nil)
			return nil
		}

		// We buffer the message to send it several times
		bufferedMessage, err := buffering.CopyMessage(ctx, message, transformers...)
		if err != nil {
			return err
		}
		// We don't need the original message anymore
		_ = message.Finish(nil)
		var te channel.TypeExtractorTransformer
		for _, tr := range transformers {
			switch tr.(type) {
			case channel.TypeExtractorTransformer:
				te = tr.(channel.TypeExtractorTransformer)
			}
		}

		reportArgs := channel.ReportArgs{}
		eventType, err := types.ToString(te)
		if err != nil{
			reportArgs.EventType = eventType
		}
		reportArgs.Ns = ref.Namespace
		totalDispatchTime, err := f.dispatch(ctx, bufferedMessage, additionalHeaders)
		if totalDispatchTime > channel.NoDuration {
			_ = f.reporter.ReportEventDispatchTime(&reportArgs, 0, totalDispatchTime)
		}
		return err
	}
}

func (f *MessageHandler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	f.receiver.ServeHTTP(response, request)
}

// dispatch takes the event, fans it out to each subscription in f.config. If all the fanned out
// events return successfully, then return nil. Else, return an error.
func (f *MessageHandler) dispatch(ctx context.Context, bufferedMessage binding.Message, additionalHeaders nethttp.Header) (time.Duration, error) {
	subs := len(f.config.Subscriptions)

	// Bind the lifecycle of the buffered message to the number of subs
	bufferedMessage = buffering.WithAcksBeforeFinish(bufferedMessage, subs)

	errorCh := make(chan dispatchResult, subs)
	for _, sub := range f.config.Subscriptions {
		go func(s Subscription) {
			dispatchedResultPerSub, err := f.makeFanoutRequest(ctx, bufferedMessage, additionalHeaders, s)
			errorCh <- dispatchResult{err: err, info: dispatchedResultPerSub}
		}(sub)
	}

	var totalDispatchTimeForFanout time.Duration = channel.NoDuration
	for range f.config.Subscriptions {
		select {
		case dispatchResult := <-errorCh:
			if dispatchResult.err != nil {
				f.logger.Error("Fanout had an error", zap.Error(dispatchResult.err))
				if dispatchResult.info.Time > channel.NoDuration {
					if totalDispatchTimeForFanout > channel.NoDuration {
						totalDispatchTimeForFanout += dispatchResult.info.Time
					} else {
						totalDispatchTimeForFanout = dispatchResult.info.Time
					}
				}
				return totalDispatchTimeForFanout, dispatchResult.err
			}
		case <-time.After(f.timeout):
			f.logger.Error("Fanout timed out")
			return totalDispatchTimeForFanout, errors.New("fanout timed out")
		}
	}
	// All Subscriptions returned err = nil.
	return totalDispatchTimeForFanout, nil
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (f *MessageHandler) makeFanoutRequest(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, sub Subscription) (channel.DispatchExecutionInfo, error) {
	return f.dispatcher.DispatchMessageWithRetries(
		ctx,
		message,
		additionalHeaders,
		sub.Subscriber,
		sub.Reply,
		sub.DeadLetter,
		sub.RetryConfig,
	)
}
type dispatchResult struct {
	err  error
	info channel.DispatchExecutionInfo
}

