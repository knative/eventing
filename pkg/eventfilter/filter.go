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
	PassFilter FilterResult = "pass"
	FailFilter FilterResult = "fail"
	NoFilter   FilterResult = "no_filter"
)

// FilterResult has the result of the filtering operation.
type FilterResult string

func (x FilterResult) And(y FilterResult) FilterResult {
	if x == NoFilter {
		return y
	}
	if y == NoFilter {
		return x
	}
	if x == PassFilter && y == PassFilter {
		return PassFilter
	}
	return FailFilter
}

// Filter is an interface representing an event filter of the trigger filter
type Filter interface {
	// Filter compute the predicate on the provided event and returns the result of the matching
	Filter(ctx context.Context, event cloudevents.Event) FilterResult
}

// Filters is a wrapper that runs each filter and performs the and
type Filters []Filter

func (filters Filters) Filter(ctx context.Context, event cloudevents.Event) FilterResult {
	res := NoFilter
	for _, f := range filters {
		res = res.And(f.Filter(ctx, event))
		// Short circuit to optimize it
		if res == FailFilter {
			return FailFilter
		}
	}
	return res
}

var _ Filter = Filters{}
