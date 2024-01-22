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
// It will normally be used in conjunction with multichannelfanout.EventHandler, which contains multiple
// fanout.EventHandler, each corresponding to a single Knative Channel.
package fanout

import (
	"context"
	"errors"
	nethttp "net/http"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/kncloudevents"
)

const (
	defaultTimeout = 15 * time.Minute
)

type Subscription struct {
	Subscriber     duckv1.Addressable
	Reply          *duckv1.Addressable
	DeadLetter     *duckv1.Addressable
	RetryConfig    *kncloudevents.RetryConfig
	ServiceAccount *types.NamespacedName
}

// Config for a fanout.EventHandler.
type Config struct {
	Subscriptions []Subscription `json:"subscriptions"`
	// Deprecated: AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	AsyncHandler bool `json:"asyncHandler,omitempty"`
}

// EventHandler is an http.Handler but has methods for managing
// the fanout Subscriptions. Get/Set methods are synchronized, and
// GetSubscriptions returns a copy of the Subscriptions, so you can
// use it to fetch a snapshot and use it after that safely.
type EventHandler interface {
	nethttp.Handler
	SetSubscriptions(ctx context.Context, subs []Subscription)
	GetSubscriptions(ctx context.Context) []Subscription
}

// FanoutEventHandler is a http.Handler that takes a single request in and fans it out to N other servers.
type FanoutEventHandler struct {
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	asyncHandler bool

	subscriptionsMutex sync.RWMutex
	subscriptions      []Subscription

	receiver *channel.EventReceiver

	eventDispatcher *kncloudevents.Dispatcher

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	reporter         channel.StatsReporter
	logger           *zap.Logger
	eventTypeHandler *eventtype.EventTypeAutoHandler
	channelRef       *duckv1.KReference
	channelUID       *types.UID
	hasHttpSubs      bool
	hasHttpsSubs     bool
}

// NewFanoutEventHandler creates a new fanout.EventHandler.
func NewFanoutEventHandler(
	logger *zap.Logger,
	config Config,
	reporter channel.StatsReporter,
	eventTypeHandler *eventtype.EventTypeAutoHandler,
	channelRef *duckv1.KReference,
	channelUID *types.UID,
	eventDispatcher *kncloudevents.Dispatcher,
	receiverOpts ...channel.EventReceiverOptions,
) (*FanoutEventHandler, error) {
	handler := &FanoutEventHandler{
		logger:           logger,
		timeout:          defaultTimeout,
		reporter:         reporter,
		asyncHandler:     config.AsyncHandler,
		eventTypeHandler: eventTypeHandler,
		channelRef:       channelRef,
		channelUID:       channelUID,
		eventDispatcher:  eventDispatcher,
	}

	handler.SetSubscriptions(context.Background(), config.Subscriptions)

	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	receiver, err := channel.NewEventReceiver(createEventReceiverFunction(handler), logger, reporter, receiverOpts...)
	if err != nil {
		return nil, err
	}
	handler.receiver = receiver

	return handler, nil
}

func SubscriberSpecToFanoutConfig(sub eventingduckv1.SubscriberSpec) (*Subscription, error) {
	destination := duckv1.Addressable{
		URL:      sub.SubscriberURI,
		CACerts:  sub.SubscriberCACerts,
		Audience: sub.SubscriberAudience,
	}

	var reply *duckv1.Addressable
	if sub.ReplyURI != nil {
		reply = &duckv1.Addressable{
			URL:      sub.ReplyURI,
			CACerts:  sub.ReplyCACerts,
			Audience: sub.ReplyAudience,
		}
	}

	var deadLetter *duckv1.Addressable
	if sub.Delivery != nil && sub.Delivery.DeadLetterSink != nil && sub.Delivery.DeadLetterSink.URI != nil {
		// Subscription reconcilers resolves the URI.
		deadLetter = &duckv1.Addressable{
			URL:      sub.Delivery.DeadLetterSink.URI,
			CACerts:  sub.Delivery.DeadLetterSink.CACerts,
			Audience: sub.Delivery.DeadLetterSink.Audience,
		}
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

func (f *FanoutEventHandler) SetSubscriptions(ctx context.Context, subs []Subscription) {
	f.subscriptionsMutex.Lock()
	defer f.subscriptionsMutex.Unlock()
	s := make([]Subscription, len(subs))
	copy(s, subs)
	f.subscriptions = s

	for _, sub := range f.subscriptions {
		if sub.Subscriber.URL != nil && sub.Subscriber.URL.Scheme == "https" {
			f.hasHttpsSubs = true
		} else {
			f.hasHttpSubs = true
		}
	}
}

func (f *FanoutEventHandler) GetSubscriptions(ctx context.Context) []Subscription {
	ret, _, _ := f.getSubscriptionsWithScheme()
	return ret
}

func (f *FanoutEventHandler) getSubscriptionsWithScheme() (ret []Subscription, hasHttpSubs bool, hasHttpsSubs bool) {
	f.subscriptionsMutex.RLock()
	defer f.subscriptionsMutex.RUnlock()
	ret = make([]Subscription, len(f.subscriptions))
	copy(ret, f.subscriptions)
	return ret, f.hasHttpSubs, f.hasHttpsSubs
}

func (f *FanoutEventHandler) autoCreateEventType(ctx context.Context, evnt event.Event) {
	if f.channelRef == nil {
		f.logger.Warn("No addressable for channel")
		return
	} else {
		if f.channelUID == nil {
			f.logger.Warn("No channelUID provided, unable to autocreate event type")
			return
		}
		err := f.eventTypeHandler.AutoCreateEventType(ctx, &evnt, f.channelRef, *f.channelUID)
		if err != nil {
			f.logger.Warn("EventTypeCreate failed")
			return
		}
	}
}

func createEventReceiverFunction(f *FanoutEventHandler) func(context.Context, channel.ChannelReference, event.Event, nethttp.Header) error {
	if f.asyncHandler {
		return func(ctx context.Context, ref channel.ChannelReference, evnt event.Event, additionalHeaders nethttp.Header) error {
			if f.eventTypeHandler != nil {
				f.autoCreateEventType(ctx, evnt)
			}

			subs, hasHttpSubs, hasHttpsSubs := f.getSubscriptionsWithScheme()

			if len(subs) == 0 {
				// Nothing to do here
				return nil
			}

			parentSpan := trace.FromContext(ctx)
			reportArgs := channel.ReportArgs{}
			reportArgs.EventType = evnt.Type()
			reportArgs.Ns = ref.Namespace

			go func(e event.Event, h nethttp.Header, s *trace.Span, r *channel.StatsReporter, args *channel.ReportArgs) {
				// Run async dispatch with background context.
				ctx = trace.NewContext(context.Background(), s)
				h.Set(apis.KnNamespaceHeader, ref.Namespace)
				// Any returned error is already logged in f.dispatch().
				dispatchResultForFanout := f.dispatch(ctx, subs, e, h)
				_ = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, *r, *args)
				// If there are both http and https subscribers, we need to report the metrics for both of the type
				if hasHttpSubs {
					reportArgs.EventScheme = "http"
					_ = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, *r, *args)
				}
				if hasHttpsSubs {
					reportArgs.EventScheme = "https"
					_ = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, *r, *args)
				}
			}(evnt, additionalHeaders, parentSpan, &f.reporter, &reportArgs)
			return nil
		}
	}
	return func(ctx context.Context, ref channel.ChannelReference, event event.Event, additionalHeaders nethttp.Header) error {
		if f.eventTypeHandler != nil {
			f.autoCreateEventType(ctx, event)
		}

		subs, hasHttpSubs, hasHttpsSubs := f.getSubscriptionsWithScheme()
		if len(subs) == 0 {
			// Nothing to do here
			return nil
		}

		reportArgs := channel.ReportArgs{}
		reportArgs.EventType = event.Type()
		reportArgs.Ns = ref.Namespace

		additionalHeaders.Set(apis.KnNamespaceHeader, ref.Namespace)
		dispatchResultForFanout := f.dispatch(ctx, subs, event, additionalHeaders)
		err := ParseDispatchResultAndReportMetrics(dispatchResultForFanout, f.reporter, reportArgs)
		// If there are both http and https subscribers, we need to report the metrics for both of the type
		// In this case we report http metrics because above we checked first for https and reported it so the left over metric to report is for http
		if hasHttpSubs {
			reportArgs.EventScheme = "http"
			err = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, f.reporter, reportArgs)
		}
		if hasHttpsSubs {
			reportArgs.EventScheme = "https"
			err = ParseDispatchResultAndReportMetrics(dispatchResultForFanout, f.reporter, reportArgs)
		}
		return err
	}
}

func (f *FanoutEventHandler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	f.receiver.ServeHTTP(response, request)
}

// ParseDispatchResultAndReportMetric processes the dispatch result and records the related channel metrics with the appropriate context
func ParseDispatchResultAndReportMetrics(result DispatchResult, reporter channel.StatsReporter, reportArgs channel.ReportArgs) error {
	if result.info != nil && result.info.Duration > kncloudevents.NoDuration {
		if result.info.ResponseCode > kncloudevents.NoResponse {
			_ = reporter.ReportEventDispatchTime(&reportArgs, result.info.ResponseCode, result.info.Duration)
		} else {
			_ = reporter.ReportEventDispatchTime(&reportArgs, nethttp.StatusInternalServerError, result.info.Duration)
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
func (f *FanoutEventHandler) dispatch(ctx context.Context, subs []Subscription, event event.Event, additionalHeaders nethttp.Header) DispatchResult {
	errorCh := make(chan DispatchResult, len(subs))
	for _, sub := range subs {
		go func(s Subscription) {
			dispatchedResultPerSub, err := f.makeFanoutRequest(ctx, event, additionalHeaders, s)
			errorCh <- DispatchResult{err: err, info: dispatchedResultPerSub}
		}(sub)
	}

	var totalDispatchTimeForFanout time.Duration = kncloudevents.NoDuration
	dispatchResultForFanout := DispatchResult{
		info: &kncloudevents.DispatchInfo{
			Duration:     kncloudevents.NoDuration,
			ResponseCode: kncloudevents.NoResponse,
		},
	}
	for range subs {
		select {
		case dispatchResult := <-errorCh:
			if dispatchResult.info != nil {
				if dispatchResult.info.Duration > kncloudevents.NoDuration {
					if totalDispatchTimeForFanout > kncloudevents.NoDuration {
						totalDispatchTimeForFanout += dispatchResult.info.Duration
					} else {
						totalDispatchTimeForFanout = dispatchResult.info.Duration
					}
				}
				dispatchResultForFanout.info.Duration = totalDispatchTimeForFanout
				dispatchResultForFanout.info.ResponseCode = dispatchResult.info.ResponseCode
			}
			if dispatchResult.err != nil {
				f.logger.Error("Fanout had an error", zap.Error(dispatchResult.err))
				dispatchResultForFanout.err = dispatchResult.err
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
func (f *FanoutEventHandler) makeFanoutRequest(ctx context.Context, event event.Event, additionalHeaders nethttp.Header, sub Subscription) (*kncloudevents.DispatchInfo, error) {
	dispatchOptions := []kncloudevents.SendOption{
		kncloudevents.WithHeader(additionalHeaders),
		kncloudevents.WithReply(sub.Reply),
		kncloudevents.WithDeadLetterSink(sub.DeadLetter),
		kncloudevents.WithRetryConfig(sub.RetryConfig),
	}

	if sub.ServiceAccount != nil {
		dispatchOptions = append(dispatchOptions, kncloudevents.WithOIDCAuthentication(sub.ServiceAccount))
	}

	return f.eventDispatcher.SendEvent(ctx, event, sub.Subscriber, dispatchOptions...)
}

type DispatchResult struct {
	err  error
	info *kncloudevents.DispatchInfo
}

func (d DispatchResult) Error() error {
	return d.err
}

func (d DispatchResult) Info() *kncloudevents.DispatchInfo {
	return d.info
}

func NewDispatchResult(err error, info *kncloudevents.DispatchInfo) DispatchResult {
	return DispatchResult{
		err:  err,
		info: info,
	}
}
