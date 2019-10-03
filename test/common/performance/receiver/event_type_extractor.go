package receiver

import cloudevents "github.com/cloudevents/sdk-go"

type TypeExtractor func(event cloudevents.Event) string

func EventTypeExtractor(event cloudevents.Event) string {
	return event.Type()
}
