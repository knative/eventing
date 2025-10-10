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
	"fmt"
	nethttp "net/http"
	"sync"
	"time"

	"github.com/cloudevents/sdk-go/v2/event"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	"knative.dev/eventing/pkg/apis"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/tracing"
)

const (
	defaultTimeout = 15 * time.Minute
	ScopeName      = "knative.dev/eventing/pkg/channel/fanout"
)

var (
	latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
)

type Subscription struct {
	Subscriber     duckv1.Addressable
	Reply          *duckv1.Addressable
	DeadLetter     *duckv1.Addressable
	RetryConfig    *kncloudevents.RetryConfig
	ServiceAccount *types.NamespacedName
	Name           string
	Namespace      string
	UID            types.UID
}

// Config for a fanout.EventHandler.
type Config struct {
	Subscriptions []Subscription `json:"subscriptions"`
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	//
	// Async handler is subject to event loss since it responds with 200 before forwarding the event
	// to all subscriptions.
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

	logger           *zap.Logger
	eventTypeHandler *eventtype.EventTypeAutoHandler
	channelRef       *duckv1.KReference
	channelUID       *types.UID
	hasHttpSubs      bool
	hasHttpsSubs     bool
	dispatchDuration metric.Float64Histogram

	// eventRecorder is used to emit Kubernetes events when messages are dropped
	// (after retries are exhausted and DLS fails or is absent)
	eventRecorder record.EventRecorder
}

// NewFanoutEventHandler creates a new fanout.EventHandler.
func NewFanoutEventHandler(
	logger *zap.Logger,
	config Config,
	eventTypeHandler *eventtype.EventTypeAutoHandler,
	channelRef *duckv1.KReference,
	channelUID *types.UID,
	eventDispatcher *kncloudevents.Dispatcher,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
	receiverOpts ...channel.EventReceiverOptions,
) (*FanoutEventHandler, error) {
	handler := &FanoutEventHandler{
		logger:           logger,
		timeout:          defaultTimeout,
		asyncHandler:     config.AsyncHandler,
		eventTypeHandler: eventTypeHandler,
		channelRef:       channelRef,
		channelUID:       channelUID,
		eventDispatcher:  eventDispatcher,
	}

	if meterProvider == nil {
		meterProvider = otel.GetMeterProvider()
	}
	if traceProvider == nil {
		traceProvider = otel.GetTracerProvider()
	}

	handler.SetSubscriptions(context.Background(), config.Subscriptions)

	receiverOpts = append(
		// set these first in case they were set to something else in the passed through receiverOpts
		[]channel.EventReceiverOptions{
			channel.MeterProvider(meterProvider),
			channel.TraceProvider(traceProvider),
		},
		receiverOpts...,
	)

	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	receiver, err := channel.NewEventReceiver(createEventReceiverFunction(handler), logger, receiverOpts...)
	if err != nil {
		return nil, err
	}
	handler.receiver = receiver

	meter := meterProvider.Meter(ScopeName)
	handler.dispatchDuration, err = meter.Float64Histogram(
		"kn.eventing.dispatch.duration",
		metric.WithDescription("The duration to dispatch the event"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		return nil, err
	}

	return handler, nil
}
// SetEventRecorder sets the event recorder for emitting Kubernetes events when messages are dropped.
func (f *FanoutEventHandler) SetEventRecorder(recorder record.EventRecorder) {
	f.eventRecorder = recorder
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

	s := &Subscription{Subscriber: destination, Reply: reply, DeadLetter: deadLetter, RetryConfig: retryConfig, UID: sub.UID}

	if sub.Name != nil {
		s.Name = *sub.Name
	}

	return s, nil
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
	f.subscriptionsMutex.RLock()
	defer f.subscriptionsMutex.RUnlock()
	ret := make([]Subscription, len(f.subscriptions))
	copy(ret, f.subscriptions)
	return ret
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
		f.eventTypeHandler.AutoCreateEventType(ctx, &evnt, f.channelRef, *f.channelUID)
	}
}

func createEventReceiverFunction(f *FanoutEventHandler) func(context.Context, channel.ChannelReference, event.Event, nethttp.Header) error {
	if f.asyncHandler {
		return func(ctx context.Context, ref channel.ChannelReference, evnt event.Event, additionalHeaders nethttp.Header) error {
			if f.eventTypeHandler != nil {
				f.autoCreateEventType(ctx, evnt)
			}

			subs := f.GetSubscriptions(ctx)
			if len(subs) == 0 {
				// Nothing to do here
				return nil
			}

			parentSpan := trace.SpanFromContext(ctx)

			go func(e event.Event, h nethttp.Header, s trace.Span) {
				// Run async dispatch with background context.
				ctx = trace.ContextWithSpan(context.Background(), s)
				// Any returned error is already logged in f.dispatch().
				_ = f.dispatch(ctx, subs, e, h)

			}(evnt, additionalHeaders, parentSpan)
			return nil
		}
	}
	return func(ctx context.Context, ref channel.ChannelReference, event event.Event, additionalHeaders nethttp.Header) error {
		if f.eventTypeHandler != nil {
			f.autoCreateEventType(ctx, event)
		}

		subs := f.GetSubscriptions(ctx)
		if len(subs) == 0 {
			// Nothing to do here
			return nil
		}

		// Any returned error is already logged in f.dispatch().
		dispatchResultForFanout := f.dispatch(ctx, subs, event, additionalHeaders)
		return dispatchResultForFanout.err
	}
}

func (f *FanoutEventHandler) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	f.receiver.ServeHTTP(response, request)
}

// dispatch takes the event, fans it out to each subscription in subs. If all the fanned out
// events return successfully, then return nil. Else, return an error.
func (f *FanoutEventHandler) dispatch(ctx context.Context, subs []Subscription, event event.Event, additionalHeaders nethttp.Header) DispatchResult {
	results := make(chan DispatchResult, len(subs))
	for _, sub := range subs {
		go func(s Subscription) {
			h := additionalHeaders.Clone()
			h.Set(apis.KnNamespaceHeader, s.Namespace)

			dispatchedResultPerSub, err := f.makeFanoutRequest(ctx, event, h, s)
			r := DispatchResult{err: err, info: dispatchedResultPerSub, subscription: s}
			results <- r

			labeler, _ := otelhttp.LabelerFromContext(ctx)
			labels := append(
				observability.MessagingLabels(
					tracing.SubscriptionMessagingDestination(types.NamespacedName{Name: s.Name, Namespace: s.Namespace}),
					"send",
				),
				labeler.Get()...,
			)
			f.dispatchDuration.Record(ctx, dispatchedResultPerSub.Duration.Seconds(), metric.WithAttributes(labels...))

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
		case dispatchResult := <-results:
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
                 // Emit K8s Warning event for dropped event.
				// An error from SendEvent means retries exhausted and DLS failed/absent.
				f.emitEventDroppedWarning(ctx, event, dispatchResult.subscription, dispatchResult.err)
			
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

	if f.eventTypeHandler != nil && sub.Name != "" && sub.Namespace != "" && sub.UID != types.UID("") {
		dispatchOptions = append(dispatchOptions, kncloudevents.WithEventTypeAutoHandler(
			f.eventTypeHandler,
			&duckv1.KReference{
				Name:       sub.Name,
				Namespace:  sub.Namespace,
				APIVersion: messagingv1.SchemeGroupVersion.String(),
				Kind:       "Subscription",
			},
			sub.UID,
		))
	}

	if sub.ServiceAccount != nil {
		dispatchOptions = append(dispatchOptions, kncloudevents.WithOIDCAuthentication(sub.ServiceAccount))
	}

	return f.eventDispatcher.SendEvent(ctx, event, sub.Subscriber, dispatchOptions...)
}
// emitEventDroppedWarning creates a Kubernetes Warning event for dropped events.
// This is called when SendEvent returns an error, which indicates that:
// 1. All retry attempts have been exhausted
// 2. DeadLetterSink delivery failed or is not configured
func (f *FanoutEventHandler) emitEventDroppedWarning(ctx context.Context, event event.Event, sub Subscription, err error) {
	if f.eventRecorder == nil {
		f.logger.Warn("Event recorder not available, cannot emit K8s event for dropped message")
		return
	}

	// Skip if subscription has no proper identity
	if sub.Name == "" || sub.Namespace == "" || sub.UID == types.UID("") {
		f.logger.Warn("Subscription missing identity, cannot emit K8s event",
			zap.String("name", sub.Name),
			zap.String("namespace", sub.Namespace))
		return
	}

	// Get event metadata for proper labeling
	eventType := event.Type()
	eventSource := event.Source()
	eventID := event.ID()

	// Create the event message
	message := fmt.Sprintf("Event dropped after retries and DLS failure: type=%s, source=%s, id=%s, error=%v",
		eventType, eventSource, eventID, err)

	// Create annotations with event-type and event-source for Knative reconciler.
	// These annotations enable the reconciler to properly aggregate events and bump series.count.
	annotations := map[string]string{
		"event-type":   eventType,
		"event-source": eventSource,
		"event-id":     eventID,
	}

	// Emit the Warning event.
	// Using a stable reason ("EventDropped") ensures proper event aggregation.
	// K8s will automatically aggregate events with same reason, source, and involved object.
	f.eventRecorder.AnnotatedEventf(
		&corev1.ObjectReference{
			APIVersion: messagingv1.SchemeGroupVersion.String(),
			Kind:       "Subscription",
			Name:       sub.Name,
			Namespace:  sub.Namespace,
			UID:        sub.UID,
		},
		annotations,
		corev1.EventTypeWarning,
		"EventDropped",
		message,
	)

	f.logger.Info("Emitted K8s Warning event for dropped message",
		zap.String("subscription", fmt.Sprintf("%s/%s", sub.Namespace, sub.Name)),
		zap.String("eventType", eventType),
		zap.String("eventSource", eventSource),
		zap.String("eventID", eventID),
	)
}

type DispatchResult struct {
	err          error
	info         *kncloudevents.DispatchInfo
	subscription Subscription // Added to track which subscription failed
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