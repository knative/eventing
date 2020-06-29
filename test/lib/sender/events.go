package sender

import (
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

const (
	EventType                   = "sender.test.knative.dev"
	ResponseStatusCodeExtension = "responsestatuscode"
)

func NewSenderEvent(id string, source string, event *cloudevents.Event, result *cehttp.Result) cloudevents.Event {
	ev := cloudevents.NewEvent()
	ev.SetID(id)
	ev.SetSource(source)
	ev.SetType(EventType)

	if result != nil {
		ev.SetExtension(ResponseStatusCodeExtension, result.StatusCode)
	}

	if event != nil {
		_ = ev.SetData("application/json", event)
	}

	return ev
}
