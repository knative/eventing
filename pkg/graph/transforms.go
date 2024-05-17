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

func buildRegexForAttribute(attribute string) (*regexp.Regexp, error) {
	chunks := []string{"^"}

	var chunk strings.Builder
	for i := 0; i < len(attribute); {
		if attribute[i] == '{' {
			chunks = append(chunks, chunk.String(), "[a-zA-Z]+")
			chunk.Reset()

			offset := strings.Index(attribute[i:], "}")
			if offset == -1 {
				return nil, fmt.Errorf("no closing bracket for variable")
			}

			i += offset + 1
			continue
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
