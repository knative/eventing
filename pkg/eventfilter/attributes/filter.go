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

package attributes

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
)

type attributesFilter map[string]string

// NewAttributesFilter returns an event filter which performs the exact match on the attributes
func NewAttributesFilter(attrs map[string]string) eventfilter.Filter {
	return attributesFilter(attrs)
}

func (attrs attributesFilter) Filter(ctx context.Context, event cloudevents.Event) eventfilter.FilterResult {
	// Set standard context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	ce := map[string]interface{}{
		"specversion":     event.SpecVersion(),
		"type":            event.Type(),
		"source":          event.Source(),
		"subject":         event.Subject(),
		"id":              event.ID(),
		"time":            event.Time().String(),
		"dataschema":      event.DataSchema(),
		"schemaurl":       event.DataSchema(),
		"datacontenttype": event.DataContentType(),
		"datamediatype":   event.DataMediaType(),
		// TODO: use data_base64 when SDK supports it.
		"datacontentencoding": event.DeprecatedDataContentEncoding(),
	}
	ext := event.Extensions()
	for k, v := range ext {
		ce[k] = v
	}

	for k, v := range attrs {
		var value interface{}
		value, ok := ce[k]
		// If the attribute does not exist in the event (extension context attributes) or if the event attribute
		// has an empty string value (optional attributes) - which means it was never set in the incoming event,
		// return false.
		if !ok || (v == eventingv1.TriggerAnyFilter && value == "") {
			logging.FromContext(ctx).Debug("Attribute not found", zap.String("attribute", k))
			return eventfilter.FailFilter
		}
		// If the attribute is not set to any and is different than the one from the event, return false.
		if v != eventingv1.TriggerAnyFilter && v != value {
			logging.FromContext(ctx).Debug("Attribute had non-matching value", zap.String("attribute", k), zap.String("filter", v), zap.Any("received", value))
			return eventfilter.FailFilter
		}
	}
	return eventfilter.PassFilter
}

var _ eventfilter.Filter = attributesFilter{}
