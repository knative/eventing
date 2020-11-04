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

// AllOf combines matchers together
func AllOf(matchers ...EventInfoMatcher) EventInfoMatcher {
	return func(have EventInfo) error {
		for _, m := range matchers {
			if err := m(have); err != nil {
				return err
			}
		}
		return nil
	}
}

// Matcher that fails if there is an error in the EventInfo
func NoError() EventInfoMatcher {
	return func(ei EventInfo) error {
		if ei.Error != "" {
			return fmt.Errorf("not expecting an error in event info: %s", ei.Error)
		}
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

// Convert a matcher that checks valid messages to a function
// that checks EventInfo structures, returning an error for any that don't
// contain valid events.
func HasAdditionalHeader(key, value string) EventInfoMatcher {
	key = strings.ToLower(key)
	return func(ei EventInfo) error {
		for k, v := range ei.HTTPHeaders {
			if strings.ToLower(k) == key && v[0] == value {
				return nil
			}
		}
		return fmt.Errorf("cannot find header '%s' = '%s' between the headers", key, value)
	}
}

// MatchKind matches the kind of EventInfo
func MatchKind(kind EventKind) EventInfoMatcher {
	return func(info EventInfo) error {
		if kind != info.Kind {
			return fmt.Errorf("event kind don't match. Expected: '%s', Actual: '%s'", kind, info.Kind)
		}
		return nil
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
