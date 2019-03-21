package cloudevents

// EventContext is conical interface for a CloudEvents Context.
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

	// GetSource returns the CloudEvents source from the context.
	GetSource() string

	// GetSchemaURL returns the CloudEvents schema URL (if any) from the context.
	GetSchemaURL() string

	// ExtensionAs populates 'obj' with the CloudEvents extension 'name' from the context.
	// It returns an error if the extension 'name' does not exist, the extension's type
	// does not match the 'obj' type, or if the 'obj' type is not a supported.
	ExtensionAs(name string, obj interface{}) error

	// Validate the event based on the specifics of the CloudEvents spec version
	// represented by this event context.
	Validate() error
}
