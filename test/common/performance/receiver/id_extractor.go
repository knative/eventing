package receiver

import cloudevents "github.com/cloudevents/sdk-go"

type IdExtractor func(event cloudevents.Event) string

func EventIdExtractor(event cloudevents.Event) string {
	return event.ID()
}
