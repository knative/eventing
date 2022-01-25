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
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
)

// AnyFilter runs each filter and performs an Or
type anyFilter []eventfilter.Filter

// NewAnyFilter returns an event filter which passes if any of the contained filters passes.
func NewAnyFilter(filters ...eventfilter.Filter) eventfilter.Filter {
	return append(anyFilter{}, filters...)
}

func (filter anyFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	res := eventfilter.NoFilter
	logging.FromContext(ctx).Debugw("Performing an ANY match ", zap.Any("filters", filter), zap.Any("event", event))
	for _, f := range filter {
		res = res.Or(f.Filter(ctx, event))
		// Short circuit to optimize it
		if res == eventfilter.PassFilter {
			return eventfilter.PassFilter
		}
	}
	return res
}

var _ eventfilter.Filter = &anyFilter{}
