package cloudevents

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
	"strings"
)

// Event represents the canonical representation of a CloudEvent.
type Event struct {
	Context EventContext
	Data    interface{}
}

func (e Event) DataAs(data interface{}) error {
	return datacodec.Decode(e.Context.GetDataMediaType(), e.Data, data)
}

func (e Event) SpecVersion() string {
	return e.Context.GetSpecVersion()
}

func (e Event) Type() string {
	return e.Context.GetType()
}

func (e Event) DataContentType() string {
	return e.Context.GetDataContentType()
}

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

func (e Event) String() string {
	sb := strings.Builder{}

	if s := e.SpecVersion(); s != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("SpecVersion: ")
		sb.WriteString(s)
	}

	if s := e.Type(); s != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("Type: ")
		sb.WriteString(s)
	}

	if s := e.DataContentType(); s != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("DataContentType: ")
		sb.WriteString(s)
	}

	return sb.String()
}
