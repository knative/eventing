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
	if attrs == nil {
		return eventfilter.NoFilter
	}
	for k, v := range attrs {
		value, ok := LookupAttribute(event, k)
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

func (attrs attributesFilter) Cleanup() {}

func LookupAttribute(event cloudevents.Event, attr string) (interface{}, bool) {
	// Set standard context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	switch attr {
	case "specversion":
		return event.SpecVersion(), true
	case "type":
		return event.Type(), true
	case "source":
		return event.Source(), true
	case "subject":
		return event.Subject(), true
	case "id":
		return event.ID(), true
	case "time":
		b, err := event.Time().MarshalText()
		if err != nil {
			return nil, false
		}
		return string(b), true
	case "dataschema":
		return event.DataSchema(), true
	case "schemaurl":
		return event.DataSchema(), true
	case "datacontenttype":
		return event.DataContentType(), true
	case "datamediatype":
		return event.DataMediaType(), true
	case "datacontentencoding":
		// TODO: use data_base64 when SDK supports it.
		return event.DeprecatedDataContentEncoding(), true
	default:
		val, ok := event.Extensions()[attr]
		return val, ok
	}
}

var _ eventfilter.Filter = attributesFilter{}
