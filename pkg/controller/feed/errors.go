/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package feed

const (
	// EventSourceDoesNotExist is a condition reason used when an EventSource
	// referenced by a Feed does not exist.
	EventSourceDoesNotExist string = "EventSourceDoesNotExist"

	// EventSourceDeleting is a condition reason used when an EventSource
	// referenced by a feed has been marked for deletion.
	EventSourceDeleting string = "EventSourceDeleting"

	// EventTypeDoesNotExist is a condition reason used when an EventType
	// referenced by a Feed does not exist.
	EventTypeDoesNotExist string = "EventTypeDoesNotExist"

	// EventTypeDeleting is a condition reason used when an EventType
	// referenced by a feed has been marked for deletion.
	EventTypeDeleting string = "EventTypeDeleting"
)

// StatusError can be returned from lower level functions and used as the input
// for setting conditions on the Feed.
type StatusError struct {
	// Reason is a one-word CamelCase reason for the failure.
	Reason string
	// Message is a human-readable message describing the error.
	Message string
}

// Error implements the error interface.
func (se *StatusError) Error() string {
	return se.Message
}

// EventSourceError is a StatusError specifically for EventSource errors.
type EventSourceError struct {
	StatusError
}

// Error implements the error interface.
func (es *EventSourceError) Error() string {
	return es.StatusError.Error()
}

// EventTypeError is a StatusError specifically for EventType errors.
type EventTypeError struct {
	StatusError
}

// Error implements the error interface.
func (es *EventTypeError) Error() string {
	return es.StatusError.Error()
}
