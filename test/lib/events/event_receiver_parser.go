package events

import (
	"encoding/json"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

type ReceivedEvent struct {
	Seq               uint64             `json:"seq,omitempty"`
	Event             *cloudevents.Event `json:"event,omitempty"`
	Err               error              `json:"err,omitempty"`
	AdditionalHeaders map[string]string  `json:"additionalHeaders,omitempty"`
}

func ParseReceivedEvents(t testing.TB, lines []string) []ReceivedEvent {
	events := make([]ReceivedEvent, len(lines))
	for i, l := range lines {
		var rcv ReceivedEvent
		err := json.Unmarshal([]byte(l), &rcv)
		if err != nil {
			t.Fatalf("Error while unmarshalling log line '%s': %s", l, err.Error())
		}
		events[i] = rcv
	}
	return events
}
