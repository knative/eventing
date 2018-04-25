/*
Copyright 2018 Google, Inc. All rights reserved.

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

package action

import (
	"github.com/elafros/eventing/pkg/event"
)

// The Action interface implements the capability to send events to an action of
// a particular category (e.g. Elafros routes, WebHooks, ETLs)
// Random thought: optional additional interface for SendEventAsync. We've wanted
// this in the past to allow infrastructure optimizations.
type Action interface {
	// SendEvent delivers an event to the named Action synchronously.
	// Should return the response from the action (for possible continuation)
	// or an error.
	// Returns an interface{} result which is currently unused, but will be used to
	// allow continuations in the future.
	// TODO: May need interfaces to indicate whether errors are fatal or retryable.
	SendEvent(name string, data interface{}, context *event.Context) (interface{}, error)
}

// ActionFunc allows simple Action types to be implemented as a standalone function.
type ActionFunc func(name string, data interface{}, context *event.Context) (interface{}, error)

// SendEvent implements the Action interface for a function with the same signature as Action.SendEvent.
func (a ActionFunc) SendEvent(name string, data interface{}, context *event.Context) (interface{}, error) {
	return a(name, data, context)
}
