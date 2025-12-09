package eventdrop

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// OTEL providers obtained from the global OpenTelemetry setup.
// These are used to emit metrics and trace events when events are dropped.
var (
	meter  = otel.Meter("knative.dev/eventing/eventdrop")
	tracer = otel.Tracer("knative.dev/eventing/eventdrop")

	// droppedEventsCounter is a monotonic counter that increments each time
	// an event is dropped. It uses low-cardinality labels to avoid metric explosion.
	// It is initialized to nil and only set if metric initialization succeeds.
	droppedEventsCounter metric.Int64Counter
	counterInitError     error
)

// init initializes the OpenTelemetry metric counter.
// If initialization fails, the error is stored and metrics will be skipped.
// This best-effort approach ensures that telemetry initialization failures do not
// disrupt normal Broker operation.
func init() {
	var err error
	droppedEventsCounter, err = meter.Int64Counter(
		"eventing_broker_events_dropped_total",
		metric.WithDescription("Number of events dropped by the Broker data plane"),
		metric.WithUnit("1"),
	)
	if err != nil {
		// Phase 1 does not fail on metric initialization errors.
		// Store the error in case a future phase wants to log it.
		// For now, we simply skip metrics if initialization fails; the RecordEventDropped
		// function will gracefully degrade to trace-only telemetry.
		counterInitError = err
	}
}

// recordMetrics emits a low-cardinality metric when an event is dropped.
//
// As per OpenTelemetry best practices and Evan's design review, we keep the
// label set minimal to avoid high-cardinality metric explosion:
//
//   - namespace: Kubernetes namespace of the Broker
//   - broker: Name of the Broker
//   - trigger: Name of the Trigger (may be empty at ingress)
//   - reason: Enum value explaining why the event was dropped
//
// EventType and EventSource are intentionally excluded from metrics due to their
// high cardinality. These attributes are included in traces instead, where sampling
// makes cardinality manageable.
func recordMetrics(ctx context.Context, info Info) {
	// If the counter failed to initialize, gracefully skip metrics.
	if droppedEventsCounter == nil {
		return
	}

	// Build the low-cardinality attribute set.
	attrs := []attribute.KeyValue{
		attribute.String("namespace", info.Namespace),
		attribute.String("broker", info.Broker),
		attribute.String("trigger", info.Trigger),
		attribute.String("reason", string(info.Reason)),
	}

	// Increment the dropped events counter.
	droppedEventsCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// recordTrace emits a trace event with richer, high-cardinality attributes.
//
// Traces are sampled (e.g., 1 in 1000 events), so they can safely include
// high-cardinality attributes like eventType and eventSource without impacting
// the observability backend.
//
// The function adds an "event-dropped" event to the current span if one is active.
// If no span is currently recording, the function returns early without creating
// a new span (Phase 1 design choice).
func recordTrace(ctx context.Context, info Info) {
	// Get the current span from the context.
	span := trace.SpanFromContext(ctx)

	// Only emit trace events if the span is actively recording.
	if !span.IsRecording() {
		return
	}

	// Build the full attribute set for the trace event.
	// Start with core attributes that are always present.
	attrs := []attribute.KeyValue{
		attribute.String("namespace", info.Namespace),
		attribute.String("broker", info.Broker),
		attribute.String("trigger", info.Trigger),
		attribute.String("reason", string(info.Reason)),
	}

	// Include event metadata – safe for traces due to sampling.
	// These attributes may vary widely between events, making them unsuitable
	// for metrics but ideal for sampled trace context.
	if info.EventType != "" {
		attrs = append(attrs, attribute.String("event_type", info.EventType))
	}
	if info.EventSource != "" {
		attrs = append(attrs, attribute.String("event_source", info.EventSource))
	}

	// Include optional details field if provided (e.g., "TTL=0", "loop detected").
	// This provides additional context for debugging without adding cardinality
	// to metrics.
	if info.Details != "" {
		attrs = append(attrs, attribute.String("details", info.Details))
	}

	// Add the "event-dropped" event to the current span.
	// This event, along with its attributes, will be recorded in the trace.
	span.AddEvent("event-dropped", trace.WithAttributes(attrs...))
}
