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
// therefore only attached to traces, which are sampled.
//
// Drops and rejections are counted separately, and both are per-component
// observations, not global verdicts: a Broker component rejecting an event,
// e.g. the ingress on TTL exhaustion, hands the final outcome back to its
// caller, and a Dispatcher counting a drop only knows that its own delivery
// chain, including the dead letter sink, was exhausted. In nested delivery
// chains, e.g. a Channel fanning out to a Trigger subscriber, an outer layer
// may still handle an event which an inner layer dropped, e.g. by delivering
// it to its own dead letter sink, so a single event may legitimately be
// counted by several components along its path.
package eventdrop

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/observability"
	"knative.dev/pkg/observability/attributekey"
)

const (
	// ScopeName is the instrumentation scope under which the event drop
	// telemetry is recorded.
	ScopeName = "knative.dev/eventing/pkg/eventdrop"

	droppedEventsCountName  = "kn.eventing.events.dropped"
	rejectedEventsCountName = "kn.eventing.events.rejected"
	droppedSpanName         = "kn.eventing.event.dropped"
	rejectedSpanName        = "kn.eventing.event.rejected"
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
	// failed and delivering it to the dead letter sink failed as well.
	ReasonDeadLetterSinkFailed Reason = "dead_letter_sink_failed"
)

// Labels identify the objects involved in the drop or rejection. Nil fields
// are omitted, and so are empty names and namespaces within them.
type Labels struct {
	Broker       *types.NamespacedName
	Trigger      *types.NamespacedName
	Subscription *types.NamespacedName
}

// Recorder emits telemetry about dropped and rejected events: low-cardinality
// counters and span records carrying the event details.
//
// The zero value is a no-op Recorder which only records span events on an
// already active span, and both record methods are safe to call on a nil
// *Recorder.
type Recorder struct {
	droppedEvents  metric.Int64Counter
	rejectedEvents metric.Int64Counter
	tracer         trace.Tracer
}

// NewRecorder creates a Recorder emitting metrics under the given meter
// provider and spans under the given trace provider. A counter which fails to
// initialize is omitted without disabling the other one, and nil providers
// are supported.
func NewRecorder(mp metric.MeterProvider, tp trace.TracerProvider) *Recorder {
	r := &Recorder{}
	if mp != nil {
		meter := mp.Meter(ScopeName)
		// The SDK may return a non-nil handle together with an error; only a
		// nil error guarantees the counter records, so drop failing handles.
		if dropped, err := meter.Int64Counter(
			droppedEventsCountName,
			metric.WithDescription("Number of events for which this data plane component exhausted its delivery options, e.g. neither the destination nor the dead letter sink accepted the event. Outer delivery layers may still handle the event."),
			metric.WithUnit("1"),
		); err == nil {
			r.droppedEvents = dropped
		}
		if rejected, err := meter.Int64Counter(
			rejectedEventsCountName,
			metric.WithDescription("Number of events rejected by this data plane component, e.g. by the Broker ingress on TTL exhaustion. The caller decides the final outcome."),
			metric.WithUnit("1"),
		); err == nil {
			r.rejectedEvents = rejected
		}
	}
	if tp != nil {
		r.tracer = tp.Tracer(ScopeName)
	}
	return r
}

// RecordEventDropped records that this component exhausted its delivery
// options for an event: neither the destination nor the dead letter sink
// accepted it. Callers which only reject an event and hand the final outcome
// back to their caller must use RecordEventRejected instead.
//
// The counters only carry the reason and the involved object identities. The
// event details, when available, are attached to the trace.
func (r *Recorder) RecordEventDropped(ctx context.Context, reason Reason, labels Labels, event *cloudevents.Event) {
	if r == nil {
		return
	}

	if r.droppedEvents != nil {
		r.droppedEvents.Add(ctx, 1, metric.WithAttributes(attributes(reason, labels)...))
	}
	r.recordSpan(ctx, droppedSpanName, reason, labels, event)
}

// RecordEventRejected records that a component refused an event for the given
// reason, deferring the final outcome to the caller: the event may still be
// delivered to a dead letter sink and then is not lost.
func (r *Recorder) RecordEventRejected(ctx context.Context, reason Reason, labels Labels, event *cloudevents.Event) {
	if r == nil {
		return
	}

	if r.rejectedEvents != nil {
		r.rejectedEvents.Add(ctx, 1, metric.WithAttributes(attributes(reason, labels)...))
	}
	r.recordSpan(ctx, rejectedSpanName, reason, labels, event)
}

// recordSpan makes the drop or rejection visible on the trace as a
// short-lived span of its own, attached to the trace in ctx. The span
// lifecycle is managed here instead of attaching an event to the current span,
// whose owner may end it at any time, e.g. when a request finishes while a
// delivery continues in the background.
func (r *Recorder) recordSpan(ctx context.Context, name string, reason Reason, labels Labels, event *cloudevents.Event) {
	if r.tracer == nil {
		return
	}

	attrs := attributes(reason, labels)
	if event != nil {
		attrs = append(attrs,
			observability.CloudEventID.With(event.ID()),
			observability.CloudEventType.With(event.Type()),
			observability.CloudEventSource.With(event.Source()),
		)
	}

	_, span := r.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
	span.End()
}

func attributes(reason Reason, labels Labels) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		observability.DropReason.With(string(reason)),
	}
	if labels.Broker != nil {
		attrs = appendNamedAttrs(attrs,
			observability.BrokerName, labels.Broker.Name,
			observability.BrokerNamespace, labels.Broker.Namespace)
	}
	if labels.Trigger != nil {
		attrs = appendNamedAttrs(attrs,
			observability.TriggerName, labels.Trigger.Name,
			observability.TriggerNamespace, labels.Trigger.Namespace)
	}
	if labels.Subscription != nil {
		// The data plane configuration only carries the channel namespace,
		// not the namespace of the Subscription itself, so the namespace is
		// usually empty here.
		attrs = appendNamedAttrs(attrs,
			observability.SubscriptionName, labels.Subscription.Name,
			observability.SubscriptionNamespace, labels.Subscription.Namespace)
	}
	return attrs
}

func appendNamedAttrs(attrs []attribute.KeyValue, name attributekey.String, nameValue string, namespace attributekey.String, namespaceValue string) []attribute.KeyValue {
	if nameValue != "" {
		attrs = append(attrs, name.With(nameValue))
	}
	if namespaceValue != "" {
		attrs = append(attrs, namespace.With(namespaceValue))
	}
	return attrs
}
