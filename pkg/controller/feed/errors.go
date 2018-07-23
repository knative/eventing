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
	// EventSource does not exist
	EventSourceDoesNotExist string = "EventSourceDoesNotExist"

	// EventSource has been marked for deletion
	EventSourceDeleting string = "EventSourceDeleting"

	// EventType does not exist
	EventTypeDoesNotExist string = "EventTypeDoesNotExist"

	// EventType has been marked for deletion
	EventTypeDeleting string = "EventTypeDeleting"
)

// Some errors are bubbled up from the lower level functions that we want to
// then update status with. These represent those errors. They have Reason and
// Message that can be used to bubble up lower level errors into the Conditions.
type StatusError struct {
	// Reason is a one-word CamelCase reason for the failure with EventSource
	Reason string
	// Message is the human-readable message that we'll describe
	Message string
}

func (se *StatusError) Error() string {
	return se.Message
}

type EventSourceError struct {
	StatusError
}

func (es *EventSourceError) Error() string {
	return es.StatusError.Error()
}

type EventTypeError struct {
	StatusError
}

func (es *EventTypeError) Error() string {
	return es.StatusError.Error()
}
