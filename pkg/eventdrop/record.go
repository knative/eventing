package eventdrop

import "context"

// RecordEventDropped is the single entry point for recording telemetry
// whenever an event is dropped in the Broker data plane.
//
// This function records both metrics (low-cardinality) and traces (high-cardinality),
// providing visibility into drop events across different observability dimensions.
//
// # Integration Points (Phase 1.5)
//
// The following Broker handlers will invoke this function in a follow-up PR:
//
//  1. pkg/broker/filter/filter_handler.go (ReasonTTLMissing)
//     - Called when an event lacks the internal TTL extension.
//     - Typically indicates the event was not sent by the Broker and cannot
//     - be safely treated as a looped event.
//     - At this point, the Trigger name is known and should be populated.
//
//  2. pkg/broker/ingress/ingress_handler.go (ReasonTTLExhausted)
//     - Called when the TTL countdown reaches <= 0.
//     - Indicates the event has been evaluated by multiple Triggers and
//     - the TTL mechanism is breaking an event loop.
//     - At this point, the Trigger may not be known (will be populated in future phases).
//
// # Parameters
//
//	ctx: The context carrying the current span and OTEL context.
//	info: The Info struct describing the dropped event and drop reason.
//
// # Error Handling
//
// RecordEventDropped does not return errors and performs best-effort telemetry:
// - If metrics initialization failed, only traces will be recorded.
// - If the span is not recording, only metrics will be recorded.
// - If both metric and trace recording fail silently, the function returns cleanly.
//
// This design ensures that telemetry failures do not disrupt event processing.
//
// # Phase 1 Contract
//
// This function is safe to call even if OpenTelemetry is not fully configured.
// It gracefully degrades to the available observability infrastructure.
func RecordEventDropped(ctx context.Context, info Info) {
	// Record both metrics and traces.
	// Each function performs its own availability checks and gracefully
	// degrades if the underlying OTEL infrastructure is not ready.
	recordMetrics(ctx, info)
	recordTrace(ctx, info)
}
