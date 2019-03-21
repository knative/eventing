package cloudevents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
	"sort"
	"strings"
)

// Event represents the canonical representation of a CloudEvent.
type Event struct {
	Context EventContext
	Data    interface{}
}

// DataAs attempts to populate the provided data object with the event payload.
// data should be a pointer type.
func (e Event) DataAs(data interface{}) error {
	return datacodec.Decode(e.Context.GetDataMediaType(), e.Data, data)
}

// SpecVersion returns Context.GetSpecVersion()
func (e Event) SpecVersion() string {
	return e.Context.GetSpecVersion()
}

// Type returns Context.GetType()
func (e Event) Type() string {
	return e.Context.GetType()
}

// Source returns Context.GetSource()
func (e Event) Source() string {
	return e.Context.GetSource()
}

// SchemaURL returns Context.GetSchemaURL()
func (e Event) SchemaURL() string {
	return e.Context.GetSchemaURL()
}

// ExtensionAs returns Context.ExtensionAs(name, obj)
func (e Event) ExtensionAs(name string, obj interface{}) error {
	return e.Context.ExtensionAs(name, obj)
}

// DataContentType returns Context.getDataContentType()
func (e Event) DataContentType() string {
	return e.Context.GetDataContentType()
}

// Validate performs a spec based validation on this event.
// Validation is dependent on the spec version specified in the event context.
func (e Event) Validate() error {
	if e.Context == nil {
		return fmt.Errorf("every event conforming to the CloudEvents specification MUST include a context")
	}

	if err := e.Context.Validate(); err != nil {
		return err
	}

	// TODO: validate data.

	return nil
}

// String returns a pretty-printed representation of the Event.
func (e Event) String() string {
	b := strings.Builder{}

	b.WriteString("Validation: ")

	valid := e.Validate()
	if valid == nil {
		b.WriteString("valid\n")
	} else {
		b.WriteString("invalid\n")
	}
	if valid != nil {
		b.WriteString(fmt.Sprintf("Validation Error: \n%s\n", valid.Error()))
	}

	b.WriteString("Context Attributes,\n")

	var extensions map[string]interface{}

	// TODO: This impl detail should be pushed into the impl structs.

	switch e.SpecVersion() {
	case CloudEventsVersionV01:
		if ec, ok := e.Context.(EventContextV01); ok {
			b.WriteString("  cloudEventsVersion: " + ec.CloudEventsVersion + "\n")
			b.WriteString("  eventType: " + ec.EventType + "\n")
			if ec.EventTypeVersion != nil {
				b.WriteString("  eventTypeVersion: " + *ec.EventTypeVersion + "\n")
			}
			b.WriteString("  source: " + ec.Source.String() + "\n")
			b.WriteString("  eventID: " + ec.EventID + "\n")
			if ec.EventTime != nil {
				b.WriteString("  eventTime: " + ec.EventTime.String() + "\n")
			}
			if ec.SchemaURL != nil {
				b.WriteString("  schemaURL: " + ec.SchemaURL.String() + "\n")
			}
			if ec.ContentType != nil {
				b.WriteString("  contentType: " + *ec.ContentType + "\n")
			}
			extensions = ec.Extensions
		}

	case CloudEventsVersionV02:
		if ec, ok := e.Context.(EventContextV02); ok {
			b.WriteString("  specversion: " + ec.SpecVersion + "\n")
			b.WriteString("  type: " + ec.Type + "\n")
			b.WriteString("  source: " + ec.Source.String() + "\n")
			b.WriteString("  id: " + ec.ID + "\n")
			if ec.Time != nil {
				b.WriteString("  time: " + ec.Time.String() + "\n")
			}
			if ec.SchemaURL != nil {
				b.WriteString("  schemaurl: " + ec.SchemaURL.String() + "\n")
			}
			if ec.ContentType != nil {
				b.WriteString("  contenttype: " + *ec.ContentType + "\n")
			}
			extensions = ec.Extensions
		}

	case CloudEventsVersionV03:
		if ec, ok := e.Context.(EventContextV03); ok {
			b.WriteString("  specversion: " + ec.SpecVersion + "\n")
			b.WriteString("  type: " + ec.Type + "\n")
			b.WriteString("  source: " + ec.Source.String() + "\n")
			b.WriteString("  id: " + ec.ID + "\n")
			if ec.Time != nil {
				b.WriteString("  time: " + ec.Time.String() + "\n")
			}
			if ec.SchemaURL != nil {
				b.WriteString("  schemaurl: " + ec.SchemaURL.String() + "\n")
			}
			if ec.DataContentType != nil {
				b.WriteString("  datacontenttype: " + *ec.DataContentType + "\n")
			}
			extensions = ec.Extensions
		}
	default:
		b.WriteString(e.String() + "\n")
	}

	if extensions != nil && len(extensions) > 0 {
		b.WriteString("Extensions,\n")
		keys := make([]string, 0, len(extensions))
		for k := range extensions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("  %s: %v\n", key, extensions[key]))
		}
	}

	if e.Data != nil {
		b.WriteString("Data,\n  ")
		if strings.HasPrefix(e.DataContentType(), "application/json") {
			var prettyJSON bytes.Buffer
			err := json.Indent(&prettyJSON, e.Data.([]byte), "  ", "  ")
			if err != nil {
				b.Write(e.Data.([]byte))
			} else {
				b.Write(prettyJSON.Bytes())
			}
		} else {
			b.Write(e.Data.([]byte))
		}
		b.WriteString("\n")
	}
	return b.String()
}
