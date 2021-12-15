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
	attribute, prefix string
}

// NewPrefixFilter returns an event filter which passes if the value of the context
// attribute in the CloudEvent is prefixed with prefix.
func NewPrefixFilter(attribute, prefix string) eventfilter.Filter {
	return &prefixFilter{
		attribute: attribute,
		prefix:    prefix,
	}
}

func (f *prefixFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	logger := logging.FromContext(ctx)
	logger.Debugw("Performing a prefix match ", zap.String("attribute", f.attribute), zap.String("prefix", f.prefix),
		zap.Any("event", event))
	value, ok := attributes.LookupAttribute(event, f.attribute)
	if !ok {
		logger.Debugw("Couldn't find attribute in event. Prefix match failed.", zap.String("attribute", f.attribute), zap.String("prefix", f.prefix),
			zap.Any("event", event))
		return eventfilter.FailFilter
	}

	if strings.HasPrefix(fmt.Sprintf("%v", value), f.prefix) {
		return eventfilter.PassFilter
	}
	return eventfilter.FailFilter
}
