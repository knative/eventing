package transport

type Message interface {
	// CloudEventsVersion returns the version of the CloudEvent.
	CloudEventsVersion() string

	// TODO maybe get encoding
}
