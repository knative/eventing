package event

type MutationFn func(Event) (Event, error)

// Mode of encoding.
const (
	DefaultMode    = ""
	BinaryMode     = "binary"
	StructuredMode = "structured"
)

type Event struct {
	Mode                string            `yaml:"Mode,omitempty"`
	Attributes          ContextAttributes `yaml:"ContextAttributes"`
	TransportExtensions Extensions        `yaml:"TransportExtensions,omitempty"`
	Data                string            `yaml:"Data"`
	// TODO: add support for data_base64
}

type ContextAttributes struct {
	SpecVersion string `yaml:"specversion,omitempty"`
	Type        string `yaml:"type,omitempty"`
	Time        string `yaml:"time,omitempty"`
	ID          string `yaml:"id,omitempty"`
	Source      string `yaml:"source,omitempty"`
	Subject     string `yaml:"subject,omitempty"`
	// SchemaURL replaced by DataSchema in 1.0
	SchemaURL  string `yaml:"schemaurl,omitempty"`
	DataSchema string `yaml:"dataschema,omitempty"`
	// DataContentEncoding removed in 1.0
	DataContentEncoding string     `yaml:"dataecontentncoding,omitempty"`
	DataContentType     string     `yaml:"datacontenttype,omitempty"`
	Extensions          Extensions `yaml:"Extensions,omitempty"`
}

type Extensions map[string]string
