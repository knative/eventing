package nats

type Encoding int32

const (
	Default Encoding = iota
	StructuredV02
	StructuredV03
	Unknown
)

func (e Encoding) String() string {
	switch e {
	case Default:
		return "Default Encoding " + e.Version()

	// Structured
	case StructuredV02:
		fallthrough
	case StructuredV03:
		return "Structured Encoding " + e.Version()

	default:
		return "Unknown Encoding"
	}
}

func (e Encoding) Version() string {
	switch e {

	// Version 0.2
	case Default: // <-- Move when a new default is wanted.
		fallthrough
	case StructuredV02:
		return "v0.2"

	// Version 0.3
	case StructuredV03:
		return "v0.3"

	// Unknown
	default:
		return "Unknown"
	}
}
