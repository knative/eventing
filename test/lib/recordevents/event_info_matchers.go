/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package recordevents

import (
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
)

// Does the provided EventInfo match some criteria
type EventInfoMatcher func(EventInfo) error

// Matcher that never fails
func Any() EventInfoMatcher {
	return func(ei EventInfo) error {
		return nil
	}
}

// Convert a matcher that checks valid messages to a function
// that checks EventInfo structures, returning an error for any that don't
// contain valid events.
func MatchEvent(evf ...cetest.EventMatcher) EventInfoMatcher {
	return func(ei EventInfo) error {
		if ei.Event == nil {
			return fmt.Errorf("Saw nil event")
		} else {
			return cetest.AllOf(evf...)(*ei.Event)
		}
	}
}

// MatchHeartBeatsImageMessage matches that the data field of the event, in the format of the heartbeats image, contains the following msg field
func MatchHeartBeatsImageMessage(expectedMsg string) cetest.EventMatcher {
	return cetest.AllOf(
		cetest.HasDataContentType(cloudevents.ApplicationJSON),
		func(have cloudevents.Event) error {
			var m map[string]interface{}
			err := have.DataAs(&m)
			if err != nil {
				return fmt.Errorf("cannot parse heartbeats message %s", err.Error())
			}
			if m["msg"].(string) != expectedMsg {
				return fmt.Errorf("heartbeats message don't match. Expected: '%s', Actual: '%s'", expectedMsg, m["msg"].(string))
			}
			return nil
		},
	)
}

// DataContains matches that the data field of the event, converted to a string, contains the provided string
func DataContains(expectedContainedString string) cetest.EventMatcher {
	return func(have event.Event) error {
		dataAsString := string(have.Data())
		if !strings.Contains(dataAsString, expectedContainedString) {
			return fmt.Errorf("data '%s' doesn't contain '%s'", dataAsString, expectedContainedString)
		}
		return nil
	}
}
