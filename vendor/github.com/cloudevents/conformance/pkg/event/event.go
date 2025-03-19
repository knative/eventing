/*
 Copyright 2022 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

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
	Attributes          ContextAttributes `yaml:"ContextAttributes,omitempty"`
	TransportExtensions Extensions        `yaml:"TransportExtensions,omitempty"`
	Data                string            `yaml:"Data,omitempty"`
	// TODO: add support for data_base64
}

// Get returns the value of the attribute or extension named `key`.
func (e *Event) Get(key string) string {
	switch key {
	case "specversion":
		return e.Attributes.SpecVersion
	case "type":
		return e.Attributes.Type
	case "time":
		return e.Attributes.Time
	case "id":
		return e.Attributes.ID
	case "source":
		return e.Attributes.Source
	case "subject":
		return e.Attributes.Subject
	case "schemaurl":
		return e.Attributes.SchemaURL
	case "dataschema":
		return e.Attributes.DataSchema
	case "dataecontentncoding":
		return e.Attributes.DataContentEncoding
	case "datacontenttype":
		return e.Attributes.DataContentType
	default:
		return e.Attributes.Extensions[key]
	}
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
