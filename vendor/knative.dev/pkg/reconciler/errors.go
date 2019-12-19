/*
Copyright 2019 The Knative Authors

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

package reconciler

import (
	"fmt"
)

// New returns a ReconcilerEvent fully populated.
func New(eventtype, reason, messageFmt string, args ...interface{}) error {
	return &ReconcilerEvent{
		EventType:  eventtype,
		Reason:     reason,
		Format: messageFmt,
		Args:       args,
	}
}

// ReconcilerEvent wraps the fields required for recorders to create an Event.
type ReconcilerEvent struct {
	EventType  string
	Reason     string
	Format string
	Args       []interface{}
}

// make sure ReconcilerEvent implements error.
var _ error = (*ReconcilerEvent)(nil)

// Is returns if the target error is a ReconcilerEvent type checking that
// EventType and Reason match.
func (e *ReconcilerEvent) Is(target error) bool {
	if t, ok := target.(*ReconcilerEvent); ok {
		if t != nil && t.EventType == e.EventType && t.Reason == e.Reason {
			return true
		}
	}
	return false
}

// Error returns the string that is formed by using the format string with the
// provided args.
func (e *ReconcilerEvent) Error() string {
	return fmt.Sprintf(e.Format, e.Args...)
}
