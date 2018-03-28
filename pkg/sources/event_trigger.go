package sources

// EventTrigger is a version of v1alpha1.EventTrigger where
// parameters have been resolved.
type EventTrigger struct {
	// EventType is the type of event to be observed
	EventType string `json:"eventType"`

	// Resource is the resource(s) from which to observe events.
	Resource string `json:"resource"`

	// Service is the hostname of the service that should be observed.
	Service string `json:"service"`

	// Parameters is an opaque map of configuration needed by the EventSource.
	Parameters map[string]interface{} `json:"params"`
}
