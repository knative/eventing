/*
Copyright 2023 The Knative Authors

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

package v1beta3

import (
	"context"
	"testing"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestEventTypeValidation(t *testing.T) {
	name := "invalid type"
	et := &EventType{Spec: EventTypeSpec{}}

	want := &apis.FieldError{
		Paths:   []string{"spec.attributes.id", "spec.attributes.source", "spec.attributes.specversion", "spec.attributes.type"},
		Message: "missing field(s)",
	}

	t.Run(name, func(t *testing.T) {
		got := et.Validate(context.TODO())
		if diff := cmp.Diff(want.Error(), got.Error()); diff != "" {
			t.Error("EventType.Validate (-want, +got) =", diff)
		}
	})
}

func TestEventTypeSpecValidation(t *testing.T) {
	testSource := apis.HTTP("test-source")
	tests := []struct {
		name string
		ets  *EventTypeSpec
		want *apis.FieldError
	}{{
		name: "invalid eventtype type",
		ets:  &EventTypeSpec{},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("attributes.id", "attributes.source", "attributes.specversion", "attributes.type")
			return fe
		}(),
	}, {
		name: "valid eventtype",
		ets: &EventTypeSpec{
			Reference: &duckv1.KReference{
				APIVersion: "eventing.knative.dev/v1",
				Kind:       "Broker",
				Name:       "test-broker",
			},
			Attributes: []EventAttributeDefinition{
				{
					Name:     "type",
					Value:    "event-type",
					Required: true,
				},
				{
					Name:     "source",
					Value:    testSource.String(),
					Required: true,
				},
				{
					Name:     "specversion",
					Value:    "v1",
					Required: true,
				},
				{
					Name:     "id",
					Required: true,
				},
			},
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ets.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventTypeSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestEventTypeImmutableFields(t *testing.T) {
	testSource := apis.HTTP("test-source")
	tests := []struct {
		name     string
		current  *EventType
		original *EventType
		want     *apis.FieldError
	}{{
		name: "good (no change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		original: nil,
		want:     nil,
	}, {
		name: "bad (reference change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "original-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1beta3.EventTypeSpec}.Reference.Name:
	-: "original-broker"
	+: "test-broker"
`,
		},
	}, {
		name: "bad (attributes change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "original-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1beta3.EventTypeSpec}.Attributes[0].Value:
	-: "original-type"
	+: "test-type"
`,
		},
	}, {
		name: "bad (description change)",
		current: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
				Description: "test-description",
			},
		},
		original: &EventType{
			Spec: EventTypeSpec{
				Reference: &duckv1.KReference{
					APIVersion: "eventing.knative.dev/v1",
					Kind:       "Broker",
					Name:       "test-broker",
				},
				Attributes: []EventAttributeDefinition{
					{
						Name:     "type",
						Value:    "test-type",
						Required: true,
					},
					{
						Name:     "source",
						Value:    testSource.String(),
						Required: true,
					},
					{
						Name:     "specversion",
						Value:    "v1",
						Required: true,
					},
					{
						Name:     "id",
						Required: true,
					},
				},
				Description: "original-description",
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: `{v1beta3.EventTypeSpec}.Description:
	-: "original-description"
	+: "test-description"
`,
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.current.CheckImmutableFields(context.TODO(), test.original)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("CheckImmutableFields (-want, +got) =", diff)
			}
		})
	}
}
