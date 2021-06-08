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
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/buffering"
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

// MessageHandler is an http.Handler but has methods for managing
// the fanout Subscriptions. Get/Set methods are synchronized, and
// GetSubscriptions returns a copy of the Subscriptions, so you can
// use it to fetch a snapshot and use it after that safely.
type MessageHandler interface {
	nethttp.Handler
	SetSubscriptions(ctx context.Context, subs []Subscription)
	GetSubscriptions(ctx context.Context) []Subscription
}

// MessageHandler is a http.Handler that takes a single request in and fans it out to N other servers.
type FanoutMessageHandler struct {
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	asyncHandler bool

	subscriptionsMutex sync.RWMutex
	subscriptions      []Subscription

	receiver   *channel.MessageReceiver
	dispatcher channel.MessageDispatcher

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	reporter channel.StatsReporter
	logger   *zap.Logger
}

// NewMessageHandler creates a new fanout.MessageHandler.

func NewFanoutMessageHandler(logger *zap.Logger, messageDispatcher channel.MessageDispatcher, config Config, reporter channel.StatsReporter) (*FanoutMessageHandler, error) {
	handler := &FanoutMessageHandler{
		logger:       logger,
		dispatcher:   messageDispatcher,
		timeout:      defaultTimeout,
		reporter:     reporter,
		asyncHandler: config.AsyncHandler,
	}
	handler.subscriptions = make([]Subscription, len(config.Subscriptions))
	for i := range config.Subscriptions {
		handler.subscriptions[i] = config.Subscriptions[i]
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
		// Subscription reconcilers resolves the URI.
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

func (f *FanoutMessageHandler) SetSubscriptions(ctx context.Context, subs []Subscription) {
	f.subscriptionsMutex.Lock()
	defer f.subscriptionsMutex.Unlock()
	s := make([]Subscription, len(subs))
	copy(s, subs)
	f.subscriptions = s
}

func (f *FanoutMessageHandler) GetSubscriptions(ctx context.Context) []Subscription {
	f.subscriptionsMutex.RLock()
	defer f.subscriptionsMutex.RUnlock()
	ret := make([]Subscription, len(f.subscriptions))
	copy(ret, f.subscriptions)
	return ret
}

func createMessageReceiverFunction(f *FanoutMessageHandler) func(context.Context, channel.ChannelReference, binding.Message, []binding.Transformer, nethttp.Header) error {
	if f.asyncHandler {
		return func(ctx context.Context, ref channel.ChannelReference, message binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
			subs := f.GetSubscriptions(ctx)

			if len(subs) == 0 {
				// Nothing to do here, finish the message and return
				_ = message.Finish(nil)
				return nil
			}

			parentSpan := trace.FromContext(ctx)
			te := kncloudevents.TypeExtractorTransformer("")
			transformers = append(transformers, &te)
			// Message buffering here is done before starting the dispatch goroutine
			// Because the message could be closed before the buffering happens
			bufferedMessage, err := buffering.CopyMessage(ctx, message, transformers...)
			if err != nil {
				return err
			}

			reportArgs := channel.ReportArgs{}
			reportArgs.EventType = string(te)
			reportArgs.Ns = ref.Namespace

			// We don't need the original message anymore
			_ = message.Finish(nil)
			go func(m binding.Message, h nethttp.Header, s *trace.Span, r *channel.StatsReporter, args *channel.ReportArgs) {
				// Run async dispatch with background context.
				ctx = trace.NewContext(context.Background(), s)
				// Any returned error is already logged in f.dispatch().
				dispatchResultForFanout := f.dispatch(ctx, subs, m, h)
				_ = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, *r, *args)
			}(bufferedMessage, additionalHeaders, parentSpan, &f.reporter, &reportArgs)
			return nil
		}
	}
	return func(ctx context.Context, ref channel.ChannelReference, message binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
		subs := f.GetSubscriptions(ctx)
		if len(subs) == 0 {
			// Nothing to do here, finish the message and return
			_ = message.Finish(nil)
			return nil
		}

		te := kncloudevents.TypeExtractorTransformer("")
		transformers = append(transformers, &te)
		// We buffer the message to send it several times
		bufferedMessage, err := buffering.CopyMessage(ctx, message, transformers...)
		if err != nil {
			return err
		}
		// We don't need the original message anymore
		_ = message.Finish(nil)

		reportArgs := channel.ReportArgs{}
		reportArgs.EventType = string(te)
		reportArgs.Ns = ref.Namespace
		dispatchResultForFanout := f.dispatch(ctx, subs, bufferedMessage, additionalHeaders)
		return ParseDispatchResultAndReportMetrics(dispatchResultForFanout, f.reporter, reportArgs)
	}
}

func (f *FanoutMessageHandler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	f.receiver.ServeHTTP(response, request)
}

// ParseDispatchResultAndReportMetric processes the dispatch result and records the related channel metrics with the appropriate context
func ParseDispatchResultAndReportMetrics(result DispatchResult, reporter channel.StatsReporter, reportArgs channel.ReportArgs) error {
	if result.info != nil && result.info.Time > channel.NoDuration {
		if result.info.ResponseCode > channel.NoResponse {
			_ = reporter.ReportEventDispatchTime(&reportArgs, result.info.ResponseCode, result.info.Time)
		} else {
			_ = reporter.ReportEventDispatchTime(&reportArgs, nethttp.StatusInternalServerError, result.info.Time)
		}
	}
	err := result.err
	if err != nil {
		channel.ReportEventCountMetricsForDispatchError(err, reporter, &reportArgs)
	} else if result.info != nil {
		_ = reporter.ReportEventCount(&reportArgs, result.info.ResponseCode)
	}
	return err
}

// dispatch takes the event, fans it out to each subscription in subs. If all the fanned out
// events return successfully, then return nil. Else, return an error.
func (f *FanoutMessageHandler) dispatch(ctx context.Context, subs []Subscription, bufferedMessage binding.Message, additionalHeaders nethttp.Header) DispatchResult {
	// Bind the lifecycle of the buffered message to the number of subs
	bufferedMessage = buffering.WithAcksBeforeFinish(bufferedMessage, len(subs))

	errorCh := make(chan DispatchResult, len(subs))
	for _, sub := range subs {
		go func(s Subscription) {
			dispatchedResultPerSub, err := f.makeFanoutRequest(ctx, bufferedMessage, additionalHeaders, s)
			errorCh <- DispatchResult{err: err, info: dispatchedResultPerSub}
		}(sub)
	}

	var totalDispatchTimeForFanout time.Duration = channel.NoDuration
	dispatchResultForFanout := DispatchResult{
		info: &channel.DispatchExecutionInfo{
			Time:         channel.NoDuration,
			ResponseCode: channel.NoResponse,
		},
	}
	for range subs {
		select {
		case dispatchResult := <-errorCh:
			if dispatchResult.info != nil {
				if dispatchResult.info.Time > channel.NoDuration {
					if totalDispatchTimeForFanout > channel.NoDuration {
						totalDispatchTimeForFanout += dispatchResult.info.Time
					} else {
						totalDispatchTimeForFanout = dispatchResult.info.Time
					}
				}
				dispatchResultForFanout.info.Time = totalDispatchTimeForFanout
				dispatchResultForFanout.info.ResponseCode = dispatchResult.info.ResponseCode
			}
			if dispatchResult.err != nil {
				f.logger.Error("Fanout had an error", zap.Error(dispatchResult.err))
				dispatchResultForFanout.err = dispatchResult.err
				return dispatchResultForFanout
			}
		case <-time.After(f.timeout):
			f.logger.Error("Fanout timed out")
			dispatchResultForFanout.err = errors.New("fanout timed out")
			return dispatchResultForFanout
		}
	}
	// All Subscriptions returned err = nil.
	return dispatchResultForFanout
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (f *FanoutMessageHandler) makeFanoutRequest(ctx context.Context, message binding.Message, additionalHeaders nethttp.Header, sub Subscription) (*channel.DispatchExecutionInfo, error) {
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

type DispatchResult struct {
	err  error
	info *channel.DispatchExecutionInfo
}

func (d DispatchResult) Error() error {
	return d.err
}

func (d DispatchResult) Info() *channel.DispatchExecutionInfo {
	return d.info
}

func NewDispatchResult(err error, info *channel.DispatchExecutionInfo) DispatchResult {
	return DispatchResult{
		err:  err,
		info: info,
	}
}
