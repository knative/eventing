package eventdrop

// Reason indicates why a Broker data-plane component dropped an event.
// This enum is intentionally small for Phase 1; additional reasons can be
// added in future phases as new drop conditions are identified and instrumented.
type Reason string

const (
	// ReasonTTLMissing is used by the filter handler when an event lacks the
	// internal TTL extension. This typically means the event was not sent by
	// the Broker itself (e.g., direct ingestion) and cannot be safely treated
	// as a looped event.
	//
	// Call site: pkg/broker/filter/filter_handler.go
	ReasonTTLMissing Reason = "ttl-missing"

	// ReasonTTLExhausted is used by the ingress handler when the TTL countdown
	// has reached <= 0. This indicates the event has been processed through
	// multiple Trigger evaluations and the TTL mechanism is breaking an event loop.
	//
	// Call site: pkg/broker/ingress/ingress_handler.go
	ReasonTTLExhausted Reason = "ttl-exhausted"

	// Future reasons might include:
	// - ReasonDeadLetterFailed (Phase 2: dead-letter delivery failure)
	// - ReasonDeliveryExhausted (Phase 2: max retry attempts exceeded)
	// - ReasonInternalError (Phase 2: non-retryable internal error)
)

// Info describes the contextual metadata related to a dropped event.
// Handler code will populate these fields when invoking RecordEventDropped.
//
// Phase 1 does not require all fields to be available at all drop locations.
// For example, at ingress time, the Trigger is not yet known, so it may be empty.
// The minimal subset (Namespace, Broker, Reason) is sufficient for metrics,
// while richer data (EventType, EventSource, Trigger) will be used in traces.
//
// Fields:
//
//	Namespace: The Kubernetes namespace where the Broker resides. Required for metrics.
//	Broker: The name of the Broker that dropped the event. Required for metrics.
//	Trigger: The name of the Trigger associated with this drop (if known).
//	         May be empty at ingress; populated by filter. Important for traces.
//	EventType: The CloudEvents "type" attribute of the dropped event.
//	           Used in traces only; omitted from metrics to avoid cardinality explosion.
//	EventSource: The CloudEvents "source" attribute of the dropped event.
//	             Used in traces only; omitted from metrics to avoid cardinality explosion.
//	Reason: The Reason enum indicating why the event was dropped. Required.
//	Details: Optional additional context (e.g., "TTL=0", "loop detected").
//	         Used in traces for richer debugging information.
type Info struct {
	Namespace   string
	Broker      string
	Trigger     string
	EventType   string
	EventSource string
	Reason      Reason
	Details     string // Optional: rich context for traces, not metrics.
}
