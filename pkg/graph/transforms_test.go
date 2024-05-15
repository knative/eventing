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
	"testing"

	"github.com/stretchr/testify/assert"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingv1beta3 "knative.dev/eventing/pkg/apis/eventing/v1beta3"
)

func TestAttributeFilterTransform(t *testing.T) {
	tests := []struct {
		name             string
		input            *eventingv1beta3.EventType
		expected         *eventingv1beta3.EventType
		filterAttributes eventingv1.TriggerFilterAttributes
	}{
		{
			name: "one attribute, none set before",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: make([]eventingv1beta3.EventAttributeDefinition, 0),
				},
			},
			expected: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
					},
				},
			},
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type": "example.event.type",
			},
		},
		{
			name: "two attributes, one set before",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
					},
				},
			},
			expected: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
						{
							Name:     "source",
							Value:    "/sample/source",
							Required: true,
						},
					},
				},
			},
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type":   "example.event.type",
				"source": "/sample/source",
			},
		},
		{
			name: "one attribute, not compatible with filter",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
					},
				},
			},
			expected: nil,
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type": "sample.event.type",
			},
		},
		{
			name: "one attribute, with variable",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.{variable}.type",
							Required: true,
						},
					},
				},
			},
			expected: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
					},
				},
			},
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type": "example.event.type",
			},
		},
		{
			name: "one attribute, with variable at end",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.{variable}",
							Required: true,
						},
					},
				},
			},
			expected: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.event.type",
							Required: true,
						},
					},
				},
			},
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type": "example.event.type",
			},
		},
		{
			name: "one attribute, with invalid variable (no closing bracket)",
			input: &eventingv1beta3.EventType{
				Spec: eventingv1beta3.EventTypeSpec{
					Attributes: []eventingv1beta3.EventAttributeDefinition{
						{
							Name:     "type",
							Value:    "example.{variable.type",
							Required: true,
						},
					},
				},
			},
			expected: nil,
			filterAttributes: eventingv1.TriggerFilterAttributes{
				"type": "example.event.type",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transform := AttributesFilterTransform{Filter: &eventingv1.TriggerFilter{Attributes: test.filterAttributes}}
			out, _ := transform.Apply(test.input, TransformFunctionContext{})
			assert.Equal(t, test.expected, out)
		})
	}
}
