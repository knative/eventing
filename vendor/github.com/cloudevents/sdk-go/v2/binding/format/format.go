package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudevents/sdk-go/v2/event"
)

// Format marshals and unmarshals structured events to bytes.
type Format interface {
	// MediaType identifies the format
	MediaType() string
	// Marshal event to bytes
	Marshal(*event.Event) ([]byte, error)
	// Unmarshal bytes to event
	Unmarshal([]byte, *event.Event) error
}

// UnknownFormat allows an event with an unknown format string to be forwarded,
// but Marshal() and Unmarshal will always fail.
type UnknownFormat string

func (uf UnknownFormat) MediaType() string                    { return string(uf) }
func (uf UnknownFormat) Marshal(*event.Event) ([]byte, error) { return nil, unknown(uf.MediaType()) }
func (uf UnknownFormat) Unmarshal([]byte, *event.Event) error { return unknown(uf.MediaType()) }

// Prefix for event-format media types.
const Prefix = "application/cloudevents"

// IsFormat returns true if mediaType begins with "application/cloudevents"
func IsFormat(mediaType string) bool { return strings.HasPrefix(mediaType, Prefix) }

// JSON is the built-in "application/cloudevents+json" format.
var JSON = jsonFmt{}

type jsonFmt struct{}

func (jsonFmt) MediaType() string { return event.ApplicationCloudEventsJSON }

func (jsonFmt) Marshal(e *event.Event) ([]byte, error) { return json.Marshal(e) }
func (jsonFmt) Unmarshal(b []byte, e *event.Event) error {
	err := json.Unmarshal(b, e)
	if err != nil {
		return err
	}

	// Extensions to go types when unparsed
	for k, v := range e.Extensions() {
		var vParsed interface{}
		switch v.(type) {
		case json.RawMessage:
			err = json.Unmarshal(v.(json.RawMessage), &vParsed)
			if err != nil {
				return err
			}
			e.SetExtension(k, vParsed)
		}
	}

	return nil
}

// built-in formats
var formats map[string]Format

func init() {
	formats = map[string]Format{}
	Add(JSON)
}

// Lookup returns the format for mediaType, or nil if not found.
func Lookup(mediaType string) Format { return formats[mediaType] }

func unknown(mediaType string) error {
	return fmt.Errorf("unknown event format media-type %#v", mediaType)
}

// Add a new Format. It can be retrieved by Lookup(f.MediaType())
func Add(f Format) { formats[f.MediaType()] = f }

// Marshal an event to bytes using the mediaType event format.
func Marshal(mediaType string, e *event.Event) ([]byte, error) {
	if f := formats[mediaType]; f != nil {
		return f.Marshal(e)
	}
	return nil, unknown(mediaType)
}

// Unmarshal bytes to an event using the mediaType event format.
func Unmarshal(mediaType string, b []byte, e *event.Event) error {
	if f := formats[mediaType]; f != nil {
		return f.Unmarshal(b, e)
	}
	return unknown(mediaType)
}
