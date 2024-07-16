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

package eventfilter

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

const (
	PassFilter FilterResult = iota
	FailFilter
	NoFilter
)

// FilterResult has the result of the filtering operation.
type FilterResult int

func (x FilterResult) String() string {
	switch x {
	case PassFilter:
		return "PassFilter"
	case FailFilter:
		return "FailFilter"
	default:
		return "NoFilter"
	}
}

// Filter is an interface representing an event filter of the trigger filter
type Filter interface {
	// Filter compute the predicate on the provided event and returns the result of the matching
	Filter(ctx context.Context, event cloudevents.Event) FilterResult
	// Cleanup cleans up any resources/goroutines used by the filter
	Cleanup()
}
