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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	// EventReason is the Kubernetes event reason used for observed events.
	CloudEventObservedReason = "CloudEventObserved"
)

// Structure to hold information about an event seen by recordevents pod.
type EventInfo struct {
	// Set if the http request received by the pod couldn't be decoded or
	// didn't pass validation
	Error string `json:"error,omitempty"`
	// Event received if the cloudevent received by the pod passed validation
	Event *cloudevents.Event `json:"event,omitempty"`
	// HTTPHeaders of the connection that delivered the event
	HTTPHeaders map[string][]string `json:"httpHeaders,omitempty"`
	Origin      string              `json:"origin,omitempty"`
	Observer    string              `json:"observer,omitempty"`
	Time        time.Time           `json:"time,omitempty"`
	Sequence    uint64              `json:"sequence"`
}

// Pretty print the event. Meant for debugging.  This formats the validation error
// or the full event as appropriate.  This does NOT format the headers.
func (ei *EventInfo) String() string {
	if ei.Event != nil {
		return ei.Event.String()
	} else {
		return fmt.Sprintf("invalid event \"%s\"", ei.Error)
	}
}

// This is mainly used for providing better failure messages
type SearchedInfo struct {
	TotalEvent int
	LastNEvent []EventInfo
}

// Pretty print the SearchedInfor for error messages
func (s *SearchedInfo) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d events seen, last %d events:", s.TotalEvent, len(s.LastNEvent)))
	for _, ei := range s.LastNEvent {
		sb.WriteString(ei.String())
		sb.WriteRune('\n')
	}
	return sb.String()
}
