/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sender

import (
	"errors"
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/event"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
)

// MatchStatusCode matches the response status code of the sent event
func MatchStatusCode(status int) cetest.EventMatcher {
	return cetest.AllOf(
		cetest.HasType(EventType),
		cetest.AnyOf(
			// Because extensions could lose type information during serialization
			// (eg when they're transported as http headers) the assert should match or the string or the int
			cetest.HasExtension(ResponseStatusCodeExtension, cetypes.FormatInteger(int32(status))),
			cetest.HasExtension(ResponseStatusCodeExtension, status),
		),
	)
}

// MatchInnerEvent matches the response event of the sent event
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
