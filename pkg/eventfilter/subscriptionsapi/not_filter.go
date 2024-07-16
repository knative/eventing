/*
Copyright 2022 The Knative Authors

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

package subscriptionsapi

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
)

type notFilter struct {
	filter eventfilter.Filter
}

// NewNotFilter returns an event filter which passes if the contained filter fails.
func NewNotFilter(f eventfilter.Filter) eventfilter.Filter {
	return &notFilter{
		filter: f,
	}
}

func (filter *notFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	if filter == nil || filter.filter == nil {
		return eventfilter.NoFilter
	}
	switch filter.filter.Filter(ctx, event) {
	case eventfilter.FailFilter:
		return eventfilter.PassFilter
	case eventfilter.PassFilter:
		return eventfilter.FailFilter
	}
	return eventfilter.NoFilter
}

func (filter *notFilter) Cleanup() {
	filter.filter.Cleanup()
}

var _ eventfilter.Filter = &notFilter{}
