/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestEventTypeValidation(t *testing.T) {
	tests := []CRDTest{{
		name: "invalid type",
		cr:   &EventType{Spec: EventTypeSpec{}},
		want: apis.ErrMissingField("type").ViaField("spec"),
	}}

	doValidateTest(t, tests)
}

func TestEventTypeImmutableFields(t *testing.T) {
	tests := []struct {
		name     string
		current  *EventType
		original *EventType
		want     *apis.FieldError
	}{{
		name: "good (no change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
				Schema: "test-schema",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
				Schema: "test-schema",
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
				Schema: "test-schema",
			},
		},
		original: nil,
		want:     nil,
	}, {
		name: "bad (broker change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "original-broker",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.EventTypeSpec}.Broker:
	-: "original-broker"
	+: "test-broker"
`,
		},
	}, {
		name: "bad (type change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:   "original-type",
				Source: "test-source",
				Broker: "test-broker",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.EventTypeSpec}.Type:
	-: "original-type"
	+: "test-type"
`,
		},
	}, {
		name: "bad (source change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "original-source",
				Broker: "test-broker",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.EventTypeSpec}.Source:
	-: "original-source"
	+: "test-source"
`,
		},
	}, {
		name: "bad (schema change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
				Schema: "test-schema",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:   "test-type",
				Source: "test-source",
				Broker: "test-broker",
				Schema: "original-schema",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.EventTypeSpec}.Schema:
	-: "original-schema"
	+: "test-schema"
`,
		},
	}, {
		name: "bad (description change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Type:        "test-type",
				Source:      "test-source",
				Broker:      "test-broker",
				Schema:      "test-schema",
				Description: "test-description",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Type:        "test-type",
				Source:      "test-source",
				Broker:      "test-broker",
				Schema:      "test-schema",
				Description: "original-description",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1alpha1.EventTypeSpec}.Description:
	-: "original-description"
	+: "test-description"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.current.CheckImmutableFields(context.TODO(), test.original)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
