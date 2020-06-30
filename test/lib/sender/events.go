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
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

const (
	EventType                   = "sender.test.knative.dev"
	ResponseStatusCodeExtension = "responsestatuscode"
)

// NewSenderEvent creates a new sender event assertable with the matchers provided in this package
func NewSenderEvent(id string, source string, event *cloudevents.Event, result *cehttp.Result) cloudevents.Event {
	ev := cloudevents.NewEvent()
	ev.SetID(id)
	ev.SetSource(source)
	ev.SetType(EventType)
	ev.SetTime(time.Now())

	if result != nil {
		ev.SetExtension(ResponseStatusCodeExtension, result.StatusCode)
	}

	if event != nil {
		_ = ev.SetData("application/json", event)
	}

	return ev
}

func NewSenderEventFromRaw(id string, source string, response *http.Response) cloudevents.Event {
	ev := cloudevents.NewEvent()
	ev.SetID(id)
	ev.SetSource(source)
	ev.SetType(EventType)
	ev.SetTime(time.Now())

	ev.SetExtension(ResponseStatusCodeExtension, response.StatusCode)

	return ev
}
