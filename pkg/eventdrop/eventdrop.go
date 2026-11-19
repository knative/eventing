/*
Copyright 2026 The Knative Authors

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

// Package eventdrop provides a centralized entry point for emitting telemetry
// when the Eventing data plane drops or rejects an event.
//
// # Telemetry design
//
// Metrics only carry low-cardinality attributes (the drop reason and the
// identities of the involved objects), because counters aggregate over all
// affected events. Event details (type, source, ID) vary per event and are
// therefore only attached to the current span, which is sampled.
//
// Drops and rejections are counted separately: a component rejecting an event,
// e.g. the Broker ingress on TTL exhaustion, hands the final outcome back to
// the caller, which may still deliver the event to a dead letter sink. Only
// components which know the event is terminally lost, e.g. the Dispatcher when
// neither the destination nor the dead letter sink could be reached, count it
// as dropped.
package eventdrop

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/observability"
)

const (
	// ScopeName is the instrumentation scope under which the event drop
	// telemetry is recorded.
	ScopeName = "knative.dev/eventing/pkg/eventdrop"

	droppedEventsCountName  = "kn.eventing.events.dropped"
	rejectedEventsCountName = "kn.eventing.events.rejected"
	droppedEventSpanEvent   = "kn.eventing.event.dropped"
	rejectedEventSpanEvent  = "kn.eventing.event.rejected"
)

// Reason describes why an event was dropped or rejected.
type Reason string

const (
	// ReasonTTLMissing is recorded when an event reaches a Broker component
	// without a valid knativebrokerttl extension, i.e. the event was not sent
	// by the Broker.
	ReasonTTLMissing Reason = "ttl_missing"

	// ReasonTTLExhausted is recorded when the TTL of an event reached zero,
	// which most likely signals an event loop.
	ReasonTTLExhausted Reason = "ttl_exhausted"

	// ReasonDeadLetterSinkFailed is recorded when the delivery of an event
	// failed and delivering it to the dead letter sink failed as well, so the
	// event could not be delivered anywhere.
	ReasonDeadLetterSinkFailed Reason = "dead_letter_sink_failed"
)

// Labels identify the objects involved in the drop or rejection. Nil fields
// are omitted.
type Labels struct {
	Broker       *types.NamespacedName
	Trigger      *types.NamespacedName
	Subscription *types.NamespacedName
}

// Recorder emits telemetry about dropped and rejected events: low-cardinality
// counters and, when the current span is recording, span events carrying the
// event details.
//
// The zero value is a no-op Recorder which only records span events, and both
// record methods are safe to call on a nil *Recorder.
type Recorder struct {
	droppedEvents  metric.Int64Counter
	rejectedEvents metric.Int64Counter
}

// NewRecorder creates a Recorder emitting metrics under the given provider.
func NewRecorder(mp metric.MeterProvider) *Recorder {
	if mp == nil {
		return &Recorder{}
	}

	meter := mp.Meter(ScopeName)
	droppedEvents, err := meter.Int64Counter(
		droppedEventsCountName,
		metric.WithDescription("Number of events dropped by the Eventing data plane, which could not be delivered anywhere"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return &Recorder{}
	}
	rejectedEvents, err := meter.Int64Counter(
		rejectedEventsCountName,
		metric.WithDescription("Number of events rejected by Eventing data plane components, e.g. by the Broker ingress on TTL exhaustion"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return &Recorder{}
	}

	return &Recorder{droppedEvents: droppedEvents, rejectedEvents: rejectedEvents}
}

// RecordEventDropped records that an event was terminally lost for the given
// reason, i.e. no component will deliver it anymore. Callers which only reject
// an event and hand the final outcome back to the caller must use
// RecordEventRejected instead, otherwise a single event may be counted more
// than once.
//
// The counters only carry the reason and the involved object identities. The
// current span, when recording, additionally gets the type, source and ID of
// the dropped event, if available.
func (r *Recorder) RecordEventDropped(ctx context.Context, reason Reason, labels Labels, event *cloudevents.Event) {
	if r == nil {
		return
	}

	if r.droppedEvents != nil {
		r.droppedEvents.Add(ctx, 1, metric.WithAttributes(attributes(reason, labels)...))
	}
	r.recordSpanEvent(ctx, droppedEventSpanEvent, reason, labels, event)
}

// RecordEventRejected records that a component refused an event for the given
// reason, deferring the final outcome to the caller: the event may still be
// delivered to a dead letter sink and is then not lost.
func (r *Recorder) RecordEventRejected(ctx context.Context, reason Reason, labels Labels, event *cloudevents.Event) {
	if r == nil {
		return
	}

	if r.rejectedEvents != nil {
		r.rejectedEvents.Add(ctx, 1, metric.WithAttributes(attributes(reason, labels)...))
	}
	r.recordSpanEvent(ctx, rejectedEventSpanEvent, reason, labels, event)
}

func (r *Recorder) recordSpanEvent(ctx context.Context, name string, reason Reason, labels Labels, event *cloudevents.Event) {
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		attrs := attributes(reason, labels)
		if event != nil {
			attrs = append(attrs,
				observability.CloudEventID.With(event.ID()),
				observability.CloudEventType.With(event.Type()),
				observability.CloudEventSource.With(event.Source()),
			)
		}
		span.AddEvent(name, trace.WithAttributes(attrs...))
	}
}

func attributes(reason Reason, labels Labels) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		observability.DropReason.With(string(reason)),
	}
	if labels.Broker != nil {
		attrs = append(attrs,
			observability.BrokerName.With(labels.Broker.Name),
			observability.BrokerNamespace.With(labels.Broker.Namespace),
		)
	}
	if labels.Trigger != nil {
		attrs = append(attrs,
			observability.TriggerName.With(labels.Trigger.Name),
			observability.TriggerNamespace.With(labels.Trigger.Namespace),
		)
	}
	if labels.Subscription != nil {
		attrs = append(attrs,
			observability.SubscriptionName.With(labels.Subscription.Name),
			observability.SubscriptionNamespace.With(labels.Subscription.Namespace),
		)
	}
	return attrs
}
