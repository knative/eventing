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

type EventKind string

const (
	EventReceived EventKind = "Received"
	EventRejected EventKind = "Rejected"

	EventSent     EventKind = "Sent"
	EventResponse EventKind = "Response"
)

// Structure to hold information about an event seen by recordevents pod.
type EventInfo struct {
	Kind EventKind `json:"kind"`

	// Set if the http request received by the pod couldn't be decoded or
	// didn't pass validation
	Error string `json:"error,omitempty"`
	// Event received if the cloudevent received by the pod passed validation
	Event *cloudevents.Event `json:"event,omitempty"`
	// In case there is a valid event in this instance, this contains only non CE headers.
	// Otherwise, it contains all the headers
	HTTPHeaders map[string][]string `json:"httpHeaders,omitempty"`
	// In case there is a valid event in this instance, this field is not filled
	Body []byte `json:"body,omitempty"`

	StatusCode int `json:"statusCode,omitempty"`

	Origin   string    `json:"origin,omitempty"`
	Observer string    `json:"observer,omitempty"`
	Time     time.Time `json:"time,omitempty"`
	Sequence uint64    `json:"sequence"`
}

// Pretty print the event. Meant for debugging.
func (ei *EventInfo) String() string {
	var sb strings.Builder
	sb.WriteString("-- EventInfo --\n")
	sb.WriteString(fmt.Sprintf("--- Kind: %v ---\n", ei.Kind))
	if ei.Event != nil {
		sb.WriteString("--- Event ---\n")
		sb.WriteString(ei.Event.String())
		sb.WriteRune('\n')
	}
	if ei.Error != "" {
		sb.WriteString("--- Error ---\n")
		sb.WriteString(ei.Error)
		sb.WriteRune('\n')
	}
	if len(ei.HTTPHeaders) != 0 {
		sb.WriteString("--- HTTP headers ---\n")
		for k, v := range ei.HTTPHeaders {
			sb.WriteString("  " + k + ": " + v[0] + "\n")
		}
		sb.WriteRune('\n')
	}
	if ei.Body != nil {
		sb.WriteString("--- Body ---\n")
		sb.Write(ei.Body)
		sb.WriteRune('\n')
	}
	if ei.StatusCode != 0 {
		sb.WriteString(fmt.Sprintf("--- Status Code: %d ---\n", ei.StatusCode))
	}
	sb.WriteString("--- Origin: '" + ei.Origin + "' ---\n")
	sb.WriteString("--- Observer: '" + ei.Observer + "' ---\n")
	sb.WriteString("--- Time: " + ei.Time.String() + " ---\n")
	sb.WriteString(fmt.Sprintf("--- Sequence: %d ---\n", ei.Sequence))
	sb.WriteString("--------------------\n")
	return sb.String()
}

// This is mainly used for providing better failure messages
type SearchedInfo struct {
	TotalEvent int
	LastNEvent []EventInfo

	storeEventsSeen    int
	storeEventsNotMine int
}

// Pretty print the SearchedInfor for error messages
func (s *SearchedInfo) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d events seen, last %d events (total events seen %d, events ignored %d):\n",
		s.TotalEvent, len(s.LastNEvent), s.storeEventsSeen, s.storeEventsNotMine))
	for _, ei := range s.LastNEvent {
		sb.WriteString(ei.String())
		sb.WriteRune('\n')
	}
	return sb.String()
}
