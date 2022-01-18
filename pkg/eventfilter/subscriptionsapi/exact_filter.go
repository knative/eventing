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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/eventfilter/attributes"

	"knative.dev/eventing/pkg/eventfilter"
)

type exactFilter struct {
	attribute, value string
	attrsFilter      eventfilter.Filter
}

// NewExactFilter returns an event filter which passes if value exactly matches the value of the context
// attribute in the CloudEvent.
func NewExactFilter(attribute, value string) (eventfilter.Filter, error) {
	if attribute == "" || value == "" {
		return nil, fmt.Errorf("invalid arguments, attribute and value can't be empty")
	}
	return &exactFilter{
		attribute: attribute,
		value:     value,
		// we're creating this filter to leverage the same filter logic of the existing attributes filter
		attrsFilter: attributes.NewAttributesFilter(map[string]string{attribute: value}),
	}, nil
}

func (filter *exactFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	if filter == nil || filter.attribute == "" || filter.value == "" {
		return eventfilter.NoFilter
	}
	logger := logging.FromContext(ctx)
	logger.Debugw("Performing an exact match ", zap.String("attribute", filter.attribute), zap.String("value", filter.value),
		zap.Any("event", event))
	return filter.attrsFilter.Filter(ctx, event)
}
