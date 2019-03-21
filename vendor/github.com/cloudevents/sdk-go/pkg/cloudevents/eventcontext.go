package cloudevents

type EventContext interface {
	// AsV01 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.1 field names, moving fields to or
	// from extensions as necessary.
	AsV01() EventContextV01

	// AsV02 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.2 field names, moving fields to or
	// from extensions as necessary.
	AsV02() EventContextV02

	// AsV03 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.3 field names, moving fields to or
	// from extensions as necessary.
	AsV03() EventContextV03

	// GetDataContentType returns content type on the context.
	GetDataContentType() string

	// GetDataMediaType returns the MIME media type for encoded data, which is
	// needed by both encoding and decoding.
	GetDataMediaType() string

	// GetSpecVersion returns the native CloudEvents Spec version of the event
	// context.
	GetSpecVersion() string

	// GetType returns the CloudEvents type from the context.
	GetType() string

	Validate() error
}
