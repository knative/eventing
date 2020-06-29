package sender

import (
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
)

func MatchStatusCode(status int) cetest.EventMatcher {
	return cetest.AllOf(cetest.HasType(EventType), cetest.HasExtension(ResponseStatusCodeExtension, status))
}

func MatchInnerEvent(matchers ...cetest.EventMatcher) cetest.EventMatcher {
	return cetest.AllOf(cetest.HasType(EventType), func(have event.Event) error {
		if have.Data() != nil {
			innerEvent := cloudevents.Event{}
			err := have.DataAs(&innerEvent)
			if err != nil {
				return fmt.Errorf("error while trying to parse inner event %w", err)
			}

			return cetest.AllOf(matchers...)(innerEvent)
		}
		return errors.New("event doesn't contain an inner event")
	})
}
