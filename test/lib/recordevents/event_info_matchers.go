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

	cetest "github.com/cloudevents/sdk-go/v2/test"
)

// Does the provided EventInfo match some criteria
type EventInfoMatcher func(EventInfo) error

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
