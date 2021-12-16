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

package v1

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
)

var (
	validEmptyTriggerFilter = newTriggerFilter(nil)
	validTriggerFilter      = newTriggerFilter(
		map[string]string{
			"type":   "other_type",
			"source": "other_source",
		})
	validSubscriptionAPIFilter = &SubscriptionsAPIFilter{
		Exact: map[string]string{
			"type": "other_type",
		},
	}
	validSubscriber = duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  "namespace",
			Name:       "subscriber_test",
			Kind:       "Service",
			APIVersion: "serving.knative.dev/v1",
		},
	}
	invalidSubscriber = duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  "namespace",
			Kind:       "Service",
			APIVersion: "serving.knative.dev/v1",
		},
	}
	// Dependency annotation
	invalidDependencyAnnotation = "invalid dependency annotation"
	dependencyAnnotationPath    = fmt.Sprintf("metadata.annotations[%s]", DependencyAnnotation)
	// Create default broker annotation
	validInjectionAnnotation   = "enabled"
	invalidInjectionAnnotation = "wut"
	injectionAnnotationPath    = fmt.Sprintf("metadata.annotations[%s]", InjectionAnnotation)
)

func TestTriggerValidation(t *testing.T) {
	invalidString := "invalid time"
	tests := []struct {
		name string
		t    *Trigger
		want *apis.FieldError
	}{{
		name: "valid injection",
		t: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "test-ns",
				Annotations: map[string]string{
					InjectionAnnotation: validInjectionAnnotation,
				}},
			Spec: TriggerSpec{
				Broker:     "default",
				Filter:     validEmptyTriggerFilter,
				Subscriber: validSubscriber,
			}},
		want: nil,
	}, {
		name: "invalid trigger spec",
		t:    &Trigger{Spec: TriggerSpec{}},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("spec.broker")
			errs = errs.Also(fe)
			fe = apis.ErrGeneric("expected at least one, got none", "spec.subscriber.ref", "spec.subscriber.uri")
			errs = errs.Also(fe)
			return errs
		}(),
	}, {
		name: "invalid dependency annotation, not a corev1.ObjectReference",
		t: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Annotations: map[string]string{
					DependencyAnnotation: invalidDependencyAnnotation,
				}},
			Spec: TriggerSpec{
				Broker:     "test_broker",
				Filter:     validEmptyTriggerFilter,
				Subscriber: validSubscriber,
			}},
		want: &apis.FieldError{
			Paths:   []string{dependencyAnnotationPath},
			Message: `The provided annotation was not a corev1.ObjectReference: "invalid dependency annotation"`,
			Details: "invalid character 'i' looking for beginning of value",
		},
	}, {
		name: "invalid dependency annotation, trigger namespace is not equal to dependency namespace)",
		t: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "test-ns-1",
				Annotations: map[string]string{
					DependencyAnnotation: `{"kind":"PingSource","namespace":"test-ns-2", "name":"test-ping-source","apiVersion":"sources.knative.dev/v1"}`,
				}},
			Spec: TriggerSpec{
				Broker:     "test_broker",
				Filter:     validEmptyTriggerFilter,
				Subscriber: validSubscriber,
			}},
		want: &apis.FieldError{
			Paths:   []string{dependencyAnnotationPath + "." + "namespace"},
			Message: `Namespace must be empty or equal to the trigger namespace "test-ns-1"`,
		},
	},
		{
			name: "invalid dependency annotation, missing kind)",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: `{"name":"test-ping-source","apiVersion":"sources.knative.dev/v1"}`,
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{dependencyAnnotationPath + "." + "kind"},
				Message: "missing field(s)",
			},
		}, {
			name: "invalid dependency annotation, missing name",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: `{"kind":"PingSource","apiVersion":"sources.knative.dev/v1"}`,
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{dependencyAnnotationPath + "." + "name"},
				Message: "missing field(s)",
			},
		}, {
			name: "invalid dependency annotation, missing apiVersion",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: `{"kind":"CronJobSource","name":"test-cronjob-source"}`,
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{dependencyAnnotationPath + "." + "apiVersion"},
				Message: "missing field(s)",
			},
		}, {
			name: "invalid dependency annotation, missing kind, name, apiVersion",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: "{}",
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths: []string{
					dependencyAnnotationPath + "." + "kind",
					dependencyAnnotationPath + "." + "name",
					dependencyAnnotationPath + "." + "apiVersion"},
				Message: "missing field(s)",
			},
		},
		{
			name: "invalid trigger spec, invalid dependency annotation(missing kind, name, apiVersion)",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: "{}",
					}},
				Spec: TriggerSpec{Subscriber: validSubscriber}},
			want: &apis.FieldError{
				Paths: []string{
					"spec.broker",
					dependencyAnnotationPath + "." + "kind",
					dependencyAnnotationPath + "." + "name",
					dependencyAnnotationPath + "." + "apiVersion"},
				Message: "missing field(s)",
			},
		}, {
			name: "invalid injection annotation value",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						InjectionAnnotation: invalidInjectionAnnotation,
					}},
				Spec: TriggerSpec{
					Broker:     "default",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{injectionAnnotationPath},
				Message: `The provided injection annotation value can only be "enabled" or "disabled", not "wut"`,
			},
		},
		{
			name: "invalid trigger spec, invalid dependency annotation(missing kind, name, apiVersion) and invalid injection",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: "{}",
						InjectionAnnotation:  invalidInjectionAnnotation,
					}},
				Spec: TriggerSpec{Broker: "default", Subscriber: validSubscriber}},
			want: func() *apis.FieldError {
				var errs *apis.FieldError
				errs = errs.Also(&apis.FieldError{
					Message: `The provided injection annotation value can only be "enabled" or "disabled", not "wut"`,
					Paths:   []string{"metadata.annotations[eventing.knative.dev/injection]"},
				})

				errs = errs.Also(apis.ErrMissingField("metadata.annotations[knative.dev/dependency].apiVersion"))
				errs = errs.Also(apis.ErrMissingField("metadata.annotations[knative.dev/dependency].kind"))
				errs = errs.Also(apis.ErrMissingField("metadata.annotations[knative.dev/dependency].name"))
				return errs
			}(),
		},
		{
			name: "valid injection annotation value, non-default broker specified",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						InjectionAnnotation: validInjectionAnnotation,
					}},
				Spec: TriggerSpec{
					Broker:     "test-broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{injectionAnnotationPath},
				Message: `The provided injection annotation is only used for default broker, but non-default broker specified here: "test-broker"`,
			},
		},
		{
			name: "invalid delivery, invalid delay string",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
				},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyTriggerFilter,
					Subscriber: validSubscriber,
					Delivery: &eventingduckv1.DeliverySpec{
						BackoffDelay: &invalidString,
					},
				}},
			want: apis.ErrInvalidValue(invalidString, "spec.delivery.backoffDelay"),
		}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.t.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("Trigger.Validate (-want, +got) =", diff)
			}
		})
	}
}

func TestTriggerUpdateValidation(t *testing.T) {
	tests := []struct {
		name string
		t    *Trigger
		tNew *Trigger
		want *apis.FieldError
	}{{
		name: "invalid update, broker changed",
		t: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "test-ns",
			},
			Spec: TriggerSpec{
				Broker:     "test_broker",
				Filter:     validEmptyTriggerFilter,
				Subscriber: validSubscriber,
			}},
		tNew: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "test-ns",
			},
			Spec: TriggerSpec{
				Broker:     "anotherBroker",
				Filter:     validEmptyTriggerFilter,
				Subscriber: validSubscriber,
			}},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "broker"},
			Details: `{string}:
	-: "test_broker"
	+: "anotherBroker"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinUpdate(context.Background(), test.t)
			got := test.tNew.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("Trigger.Validate (-want, +got) =", diff)
			}
		})
	}
}

func TestTriggerSpecValidation(t *testing.T) {
	invalidString := "invalid time"
	tests := []struct {
		name string
		ts   *TriggerSpec
		want *apis.FieldError
	}{{
		name: "invalid trigger spec",
		ts:   &TriggerSpec{},
		want: func() *apis.FieldError {
			var errs *apis.FieldError
			fe := apis.ErrMissingField("broker")
			errs = errs.Also(fe)
			fe = apis.ErrGeneric("expected at least one, got none", "subscriber.ref", "subscriber.uri")
			errs = errs.Also(fe)
			return errs

		}(),
	}, {
		name: "missing broker",
		ts: &TriggerSpec{
			Broker:     "",
			Filter:     validTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("broker")
			return fe
		}(),
	}, {
		name: "missing attributes keys, match all",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validEmptyTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "invalid attribute name, start with number",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: newTriggerFilter(
				map[string]string{
					"0invalid": "my-value",
				}),
			Subscriber: validSubscriber,
		},
		want: apis.ErrInvalidKeyName("0invalid", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("attributes", "0invalid").ViaField("filter"),
	}, {
		name: "invalid attribute name, capital letters",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: newTriggerFilter(
				map[string]string{
					"invALID": "my-value",
				}),
			Subscriber: validSubscriber,
		},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("attributes", "invALID").ViaField("filter"),
	}, {
		name: "missing subscriber",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: validTriggerFilter,
		},
		want: apis.ErrGeneric("expected at least one, got none", "subscriber.ref", "subscriber.uri"),
	}, {
		name: "missing subscriber.ref.name",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validTriggerFilter,
			Subscriber: invalidSubscriber,
		},
		want: apis.ErrMissingField("subscriber.ref.name"),
	}, {
		name: "missing broker",
		ts: &TriggerSpec{
			Broker:     "",
			Filter:     validTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: apis.ErrMissingField("broker"),
	}, {
		name: "valid empty filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validEmptyTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "valid SourceAndType filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "valid Attributes filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validTriggerFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "invalid delivery, invalid delay string",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validEmptyTriggerFilter,
			Subscriber: validSubscriber,
			Delivery: &eventingduckv1.DeliverySpec{
				BackoffDelay: &invalidString,
			},
		},
		want: apis.ErrInvalidValue(invalidString, "delivery.backoffDelay"),
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Validate TriggerSpec (-want, +got) =\n%s", diff)
			}
		})
	}
}

func TestFilterSpecValidation(t *testing.T) {
	newTriggerFiltersEnabledCtx := feature.ToContext(context.TODO(), feature.Flags{
		feature.NewTriggerFilters: feature.Enabled,
	})
	tests := []struct {
		name    string
		filter  *TriggerFilter
		filters []SubscriptionsAPIFilter
		want    *apis.FieldError
	}{{
		name:    "missing filters, match all",
		filters: nil,
		want:    &apis.FieldError{},
	}, {
		name: "invalid exact filter attribute name, start with number",
		filters: []SubscriptionsAPIFilter{
			{
				Exact: map[string]string{
					"0invalid": "some-value",
				},
			}},
		want: apis.ErrInvalidKeyName("0invalid", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("exact", "0invalid").ViaFieldIndex("filters", 0),
	}, {
		name: "invalid exact filter attribute name, capital letters",
		filters: []SubscriptionsAPIFilter{
			{
				Exact: map[string]string{
					"invALID": "some-value",
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("exact", "invALID").ViaFieldIndex("filters", 0),
	}, {
		name:   "valid empty filter",
		filter: validEmptyTriggerFilter,
		want:   &apis.FieldError{},
	}, {
		name:   "valid SourceAndType filter",
		filter: validTriggerFilter,
		want:   &apis.FieldError{},
	}, {
		name: "invalid multiple dialects",
		filters: []SubscriptionsAPIFilter{
			{
				Exact: map[string]string{
					"myext": "abc",
				},
				Suffix: map[string]string{
					"myext": "abc",
				},
			}},
		want: apis.ErrGeneric("multiple dialects found, filters can have only one dialect set"),
	}, {
		name:   "valid Attributes filter",
		filter: validTriggerFilter,
		want:   &apis.FieldError{},
	}, {
		name: "exact filter contains more than one attribute",
		filters: []SubscriptionsAPIFilter{
			{
				Exact: map[string]string{
					"myext":      "abc",
					"anotherext": "xyz",
				},
			}},
		want: apis.ErrGeneric("Multiple items found, can have only one key-value", "exact").ViaFieldIndex("filters", 0),
	}, {
		name: "valid exact filter",
		filters: []SubscriptionsAPIFilter{
			{
				Exact: map[string]string{
					"valid": "abc",
				},
			}},
		want: &apis.FieldError{},
	}, {
		name: "suffix filter contains more than one attribute",
		filters: []SubscriptionsAPIFilter{
			{
				Suffix: map[string]string{
					"myext":      "abc",
					"anotherext": "xyz",
				},
			}},
		want: apis.ErrGeneric("Multiple items found, can have only one key-value", "suffix").ViaFieldIndex("filters", 0),
	}, {
		name: "suffix filter contains invalid attribute name",
		filters: []SubscriptionsAPIFilter{
			{
				Suffix: map[string]string{
					"invALID": "abc",
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("suffix", "invALID").ViaFieldIndex("filters", 0),
	}, {
		name: "valid suffix filter",
		filters: []SubscriptionsAPIFilter{
			{
				Suffix: map[string]string{
					"valid": "abc",
				},
			}},
		want: &apis.FieldError{},
	}, {
		name: "prefix filter contains more than one attribute",
		filters: []SubscriptionsAPIFilter{
			{
				Prefix: map[string]string{
					"myext":      "abc",
					"anotherext": "xyz",
				},
			}},
		want: apis.ErrGeneric("Multiple items found, can have only one key-value", "prefix").ViaFieldIndex("filters", 0),
	}, {
		name: "prefix filter contains invalid attribute name",
		filters: []SubscriptionsAPIFilter{
			{
				Prefix: map[string]string{
					"invALID": "abc",
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("prefix", "invALID").ViaFieldIndex("filters", 0),
	}, {
		name: "valid prefix filter",
		filters: []SubscriptionsAPIFilter{
			{
				Prefix: map[string]string{
					"valid": "abc",
				},
			}},
		want: &apis.FieldError{},
	}, {
		name: "not nested expression is valid",
		filters: []SubscriptionsAPIFilter{
			{
				Not: validSubscriptionAPIFilter,
			}},
		want: &apis.FieldError{},
	}, {
		name: "not nested expression is invalid",
		filters: []SubscriptionsAPIFilter{
			{
				Not: &SubscriptionsAPIFilter{
					Prefix: map[string]string{
						"invALID": "abc",
					},
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("prefix", "invALID").ViaField("not").ViaFieldIndex("filters", 0),
	}, {
		name: "all filter is nil",
		filters: []SubscriptionsAPIFilter{
			{
				All: nil,
			}},
		want: &apis.FieldError{},
	}, {
		name: "all filter is valid",
		filters: []SubscriptionsAPIFilter{
			{
				All: []SubscriptionsAPIFilter{
					*validSubscriptionAPIFilter,
					{
						Exact: map[string]string{"myattr": "myval"},
					},
				},
			}},
		want: &apis.FieldError{},
	}, {
		name: "all filter sub expression is invalid",
		filters: []SubscriptionsAPIFilter{
			{
				All: []SubscriptionsAPIFilter{
					*validSubscriptionAPIFilter,
					{
						Exact: map[string]string{"myattr": "myval"},
					},
					{
						Prefix: map[string]string{
							"invALID": "abc",
						},
					},
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("prefix", "invALID").ViaFieldIndex("all", 2).ViaFieldIndex("filters", 0),
	}, {
		name: "any filter is valid",
		filters: []SubscriptionsAPIFilter{
			{
				Any: []SubscriptionsAPIFilter{
					*validSubscriptionAPIFilter,
					{
						Exact: map[string]string{"myattr": "myval"},
					},
				},
			}},
		want: &apis.FieldError{},
	}, {
		name: "any filter sub expression is invalid",
		filters: []SubscriptionsAPIFilter{
			{
				Any: []SubscriptionsAPIFilter{
					*validSubscriptionAPIFilter,
					{
						Exact: map[string]string{"myattr": "myval"},
					},
					{
						Prefix: map[string]string{"invALID": "abc"},
					},
				},
			}},
		want: apis.ErrInvalidKeyName("invALID", apis.CurrentField,
			"Attribute name must start with a letter and can only contain "+
				"lowercase alphanumeric").ViaFieldKey("prefix", "invALID").ViaFieldIndex("any", 2).ViaFieldIndex("filters", 0)}, {
		name: "CE SQL with syntax error",
		filters: []SubscriptionsAPIFilter{
			{
				SQL: "this is wrong",
			}},
		want: apis.ErrInvalidValue("this is wrong", "sql", "syntax error: ").ViaFieldIndex("filters", 0),
	}, {
		name: "Valid CE SQL expression",
		filters: []SubscriptionsAPIFilter{
			{
				SQL: "type = 'dev.knative' AND ttl < 3",
			}},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &TriggerSpec{
				Broker:     "test_broker",
				Filter:     test.filter,
				Filters:    test.filters,
				Subscriber: validSubscriber,
			}
			got := ts.Validate(newTriggerFiltersEnabledCtx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Validate TriggerSpec (-want, +got) =\n%s", diff)
			}
		})
	}
}

func TestTriggerImmutableFields(t *testing.T) {
	tests := []struct {
		name     string
		current  *Trigger
		original *Trigger
		want     *apis.FieldError
	}{{
		name: "good (no change)",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Subscriber: validSubscriber,
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Subscriber: validSubscriber,
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Subscriber: validSubscriber,
			},
		},
		original: nil,
		want:     nil,
	}, {
		name: "good (filter change)",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Subscriber: validSubscriber,
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Filter:     validTriggerFilter,
				Subscriber: validSubscriber,
			},
		},
		want: nil,
	}, {
		name: "bad (broker change)",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker:     "broker",
				Subscriber: validSubscriber,
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker:     "original_broker",
				Subscriber: validSubscriber,
			},
		},
		want: &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec", "broker"},
			Details: `{string}:
	-: "original_broker"
	+: "broker"
`,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = apis.WithinUpdate(ctx, test.original)
			got := test.current.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Error("CheckImmutableFields (-want, +got) =", diff)
			}
		})
	}
}

func newTriggerFilter(attrs map[string]string) *TriggerFilter {
	return &TriggerFilter{
		Attributes: attrs,
	}
}
