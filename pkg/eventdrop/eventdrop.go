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
// whenever an event is dropped by the Eventing data plane, e.g. by the Broker
// ingress on TTL exhaustion or by the Dispatcher when neither the destination
// nor the dead letter sink could be reached.
//
// # Telemetry design
//
// Metrics only carry low-cardinality attributes (the drop reason and the
// identities of the involved objects), because counters aggregate over all
// dropped events. Event details (type, source, ID) vary per event and are
// therefore only attached to the current span, which is sampled.
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
	// ScopeName is the instrumentation scope under which the dropped events
	// counter is recorded.
	ScopeName = "knative.dev/eventing/pkg/eventdrop"

	// droppedEventsCountName is the name of the counter recording dropped events.
	droppedEventsCountName = "kn.eventing.events.dropped"

	// droppedEventSpanEventName is the name of the event added to the current
	// span when an event is dropped.
	droppedEventSpanEventName = "kn.eventing.event.dropped"
)

// Reason describes why an event was dropped.
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

// Labels identify the objects involved in the drop. Nil fields are omitted.
type Labels struct {
	Broker  *types.NamespacedName
	Trigger *types.NamespacedName
}

// Recorder emits telemetry about dropped events: a low-cardinality counter and,
// when the current span is recording, a span event carrying the event details.
//
// The zero value is a no-op Recorder which only records span events, and
// RecordEventDropped is safe to call on a nil *Recorder.
type Recorder struct {
	droppedEvents metric.Int64Counter
}

// NewRecorder creates a Recorder emitting metrics under the given provider.
func NewRecorder(mp metric.MeterProvider) *Recorder {
	if mp == nil {
		return &Recorder{}
	}

	droppedEvents, err := mp.Meter(ScopeName).Int64Counter(
		droppedEventsCountName,
		metric.WithDescription("Number of events dropped by the Eventing data plane"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return &Recorder{}
	}

	return &Recorder{droppedEvents: droppedEvents}
}

// RecordEventDropped records that an event was dropped for the given reason.
//
// The counter only carries the reason and the involved object identities.
// The current span, when recording, additionally gets the type, source and ID
// of the dropped event, if available.
func (r *Recorder) RecordEventDropped(ctx context.Context, reason Reason, labels Labels, event *cloudevents.Event) {
	if r == nil {
		return
	}

	if r.droppedEvents != nil {
		r.droppedEvents.Add(ctx, 1, metric.WithAttributes(metricAttributes(reason, labels)...))
	}

	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.AddEvent(droppedEventSpanEventName, trace.WithAttributes(spanAttributes(reason, labels, event)...))
	}
}

func metricAttributes(reason Reason, labels Labels) []attribute.KeyValue {
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
	return attrs
}

func spanAttributes(reason Reason, labels Labels, event *cloudevents.Event) []attribute.KeyValue {
	attrs := metricAttributes(reason, labels)
	if event != nil {
		attrs = append(attrs,
			observability.CloudEventID.With(event.ID()),
			observability.CloudEventType.With(event.Type()),
			observability.CloudEventSource.With(event.Source()),
		)
	}
	return attrs
}
