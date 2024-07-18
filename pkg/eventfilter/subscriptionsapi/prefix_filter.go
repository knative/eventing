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
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/attributes"
)

type prefixFilter struct {
	filters map[string]string
}

// NewPrefixFilter returns an event filter which passes if the value of the context
// attribute in the CloudEvent is prefixed with prefix.
func NewPrefixFilter(filters map[string]string) (eventfilter.Filter, error) {
	for attribute, value := range filters {
		if attribute == "" || value == "" {
			return nil, fmt.Errorf("invalid arguments, attribute and prefix can't be empty")
		}
	}
	return &prefixFilter{
		filters: filters,
	}, nil
}

func (filter *prefixFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	if filter == nil {
		return eventfilter.NoFilter
	}
	logger := logging.FromContext(ctx)
	logger.Debugw("Performing a prefix match ", zap.Any("filters", filter.filters), zap.Any("event", event))
	for k, v := range filter.filters {
		value, ok := attributes.LookupAttribute(event, k)
		if !ok {
			logger.Debugw("Couldn't find attribute in event. Prefix match failed.", zap.String("attribute", k), zap.String("prefix", v),
				zap.Any("event", event))
			return eventfilter.FailFilter
		}
		var s string
		if s, ok = value.(string); !ok {
			s = fmt.Sprintf("%v", value)
		}
		if !strings.HasPrefix(s, v) {
			return eventfilter.FailFilter
		}
	}
	return eventfilter.PassFilter
}

func (filter *prefixFilter) Cleanup() {}
