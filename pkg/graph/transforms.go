/*
Copyright 2024 The Knative Authors

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

package graph

import (
	"fmt"
	"regexp"
	"strings"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

type NoTransform struct{}

var _ Transform = NoTransform{}

func (nt NoTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	return et, tfc
}

func (nt NoTransform) Name() string {
	return "no-transform"
}

type AttributesFilterTransform struct {
	Filter *eventingv1.TriggerFilter
}

var _ Transform = &AttributesFilterTransform{}

// Apply will apply the transform to a given input eventtype. This will "narrow" the eventtype to represent only events which could pass the attribute filter.
// For example, if the "type" attribute was not yet set on the eventtype, but the filter requires "type"="example.event.type", then after this filter the
// eventtype would have the attribute definition for "type"="example.event.type". Additionally, if an eventtype can not be narrowed this returns nil. For
// example using the filter from the earlier example, if the "type" was already set to "other.event.type" then the eventtype would not be compatible.
func (aft *AttributesFilterTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	etAttributes := make(map[string]*eventingv1beta3.EventAttributeDefinition)
	for i := range et.Spec.Attributes {
		etAttributes[et.Spec.Attributes[i].Name] = &et.Spec.Attributes[i]
	}

	for k, v := range aft.Filter.Attributes {
		if attribute, ok := etAttributes[k]; ok {
			if attribute.Value != v {
				regexp, err := buildRegexForAttribute(attribute.Value)
				if err != nil {
					return nil, tfc
				}

				if regexp.MatchString(v) {
					attribute.Value = v
				} else {
					return nil, tfc
				}
			}
		} else {
			etAttributes[k] = &eventingv1beta3.EventAttributeDefinition{
				Name:     k,
				Value:    v,
				Required: true,
			}
		}
	}

	updatedAttributes := make([]eventingv1beta3.EventAttributeDefinition, 0, len(et.Spec.Attributes))
	for _, v := range etAttributes {
		updatedAttributes = append(updatedAttributes, *v)
	}

	et.Spec.Attributes = updatedAttributes

	return et, tfc
}

func (aft *AttributesFilterTransform) Name() string {
	return "attributes-filter"
}

// buildRegexForAttribute build a regex which detects whether the current value for the attribute is compatible
// with the value required by the attribute filter. Specifically, it will handle variables in the eventtype so
// that if there are values that can be set in the variable so that it can match the attribute filter, those
// values will be chosen.
func buildRegexForAttribute(attribute string) (*regexp.Regexp, error) {
	chunks := []string{"^"}

	var chunk strings.Builder
	for i := 0; i < len(attribute); {
		// handle escaped curly brackets. If not escaped, treat them like part of a variable
		if attribute[i] == '\\' && i+1 < len(attribute) && (attribute[i+1] == '{' || attribute[i+1] == '}') {
			chunk.WriteByte(attribute[i+1])
			i += 2
			continue
		} else if attribute[i] == '{' {
			// this regex allows any character except those disallowed by the cloudevents spec. Technically, it is slightly more permissive
			// than the cloudevents spec, because creating a regex for code surrogates used properly in pairs is complicated
			chunks = append(chunks, chunk.String(), "[^\\x{0000}-\\x{001f}\\x{007f}-\\x{009f}\\x{fdd0}-\\x{fdef}\\x{fffe}\\x{ffff}]+")
			chunk.Reset()

			// for variable names we only allow [a-zA-Z0-9]+, so we don't need to worry about escaped characters or brackets in the variable name
			offset := strings.Index(attribute[i:], "}")
			if offset == -1 {
				return nil, fmt.Errorf("no closing bracket for variable")
			}

			i += offset + 1
			continue
		} else if attribute[i] == '}' {
			// we have an unpaired bracket
			return nil, fmt.Errorf("no opening bracket for a closing bracket. If you want to have a bracket in your value, please escape it with a \\ character")
		}

		chunk.WriteByte(attribute[i])
		i++
	}

	chunks = append(chunks, chunk.String(), "$")

	return regexp.Compile(strings.Join(chunks, ""))
}

type EventTypeTransform struct {
	EventType *eventingv1beta3.EventType
}

var _ Transform = EventTypeTransform{}

func (ett EventTypeTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	return ett.EventType.DeepCopy(), tfc
}

func (ett EventTypeTransform) Name() string {
	return "eventtype-transform"
}

type CloudEventOverridesTransform struct {
	Overrides *duckv1.CloudEventOverrides
}

var _ Transform = CloudEventOverridesTransform{}

// Apply applies the CloudEventOverrides for a given event source
func (cet CloudEventOverridesTransform) Apply(et *eventingv1beta3.EventType, tfc TransformFunctionContext) (*eventingv1beta3.EventType, TransformFunctionContext) {
	etAttributes := make(map[string]*eventingv1beta3.EventAttributeDefinition)
	for i := range et.Spec.Attributes {
		// don't use a loop var, as the memory ref would point to the loop variable rather than the element in the array
		etAttributes[et.Spec.Attributes[i].Name] = &et.Spec.Attributes[i]
	}

	for k, v := range cet.Overrides.Extensions {
		if attribute, ok := etAttributes[k]; ok {
			attribute.Value = v
		} else {
			etAttributes[k] = &eventingv1beta3.EventAttributeDefinition{
				Name:     k,
				Value:    v,
				Required: true,
			}
		}
	}

	updatedAttributes := make([]eventingv1beta3.EventAttributeDefinition, 0, len(et.Spec.Attributes))
	for _, v := range etAttributes {
		updatedAttributes = append(updatedAttributes, *v)
	}

	et.Spec.Attributes = updatedAttributes

	return et, tfc
}

func (cet CloudEventOverridesTransform) Name() string {
	return "source-ce-overrides-transform"
}
