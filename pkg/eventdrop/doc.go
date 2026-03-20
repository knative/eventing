// Package eventdrop provides a centralized entry point for capturing
// telemetry about dropped events inside the Knative Broker data plane.
//
// This package introduces the types and helpers that Broker components
// (such as the filter and ingress handlers) will call when an event is
// dropped due to TTL exhaustion or other well-defined conditions.
//
// # Phase 1 Scope
//
// Phase 1 only introduces the API surface and OpenTelemetry wiring,
// without modifying existing handler code. Integration points will be
// added in a follow-up PR once the design is reviewed and approved.
//
// The RecordEventDropped function is the single entry point for all
// drop-related telemetry, ensuring consistency across multiple components.
//
// # Telemetry Design
//
//   - Metrics: Uses low-cardinality attributes (namespace, broker, trigger, reason)
//     to avoid metric explosion. EventType and EventSource are omitted from metrics.
//
//   - Traces: Includes richer, high-cardinality attributes (eventType, eventSource)
//     since traces are sampled and can safely carry detailed context.
//
// # Future Phases
//
// Follow-up phases will build on this instrumentation to add:
// - Kubernetes Event reporting
// - Series aggregation and series.count tracking
// - Buffering and periodic reconciliation
// - Dead-letter reporting sinks
package eventdrop
