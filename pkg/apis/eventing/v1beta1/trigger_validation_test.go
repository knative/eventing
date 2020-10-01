/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1beta1

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	validEmptyFilter      = &TriggerFilter{}
	validAttributesFilter = &TriggerFilter{
		Attributes: TriggerFilterAttributes{
			"type":   "other_type",
			"source": "other_source",
		},
	}
	validSubscriber = duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  "namespace",
			Name:       "subscriber_test",
			Kind:       "Service",
			APIVersion: "serving.knative.dev/v1alpha1",
		},
	}
	invalidSubscriber = duckv1.Destination{
		Ref: &duckv1.KReference{
			Namespace:  "namespace",
			Kind:       "Service",
			APIVersion: "serving.knative.dev/v1alpha1",
		},
	}
	// Dependency annotation
	invalidDependencyAnnotation = "invalid dependency annotation"
	dependencyAnnotationPath    = fmt.Sprintf("metadata.annotations[%s]", DependencyAnnotation)
	// Create default broker annotation
	invalidInjectionAnnotation = "wut"
	injectionAnnotationPath    = fmt.Sprintf("metadata.annotations[%s]", InjectionAnnotation)
)

func TestTriggerValidation(t *testing.T) {
	tests := []struct {
		name string
		t    *Trigger
		want *apis.FieldError
	}{{
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
				Filter:     validEmptyFilter,
				Subscriber: validSubscriber,
			}},
		want: &apis.FieldError{
			Paths:   []string{dependencyAnnotationPath},
			Message: "The provided annotation was not a corev1.ObjectReference: \"invalid dependency annotation\"",
			Details: "invalid character 'i' looking for beginning of value",
		},
	}, {
		name: "invalid dependency annotation, trigger namespace is not equal to dependency namespace)",
		t: &Trigger{
			ObjectMeta: v1.ObjectMeta{
				Namespace: "test-ns-1",
				Annotations: map[string]string{
					DependencyAnnotation: "{\"kind\":\"PingSource\",\"namespace\":\"test-ns-2\", \"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1alpha1\"}",
				}},
			Spec: TriggerSpec{
				Broker:     "test_broker",
				Filter:     validEmptyFilter,
				Subscriber: validSubscriber,
			}},
		want: &apis.FieldError{
			Paths:   []string{dependencyAnnotationPath + "." + "namespace"},
			Message: "Namespace must be empty or equal to the trigger namespace \"test-ns-1\"",
		},
	},
		{
			name: "invalid dependency annotation, missing kind)",
			t: &Trigger{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test-ns",
					Annotations: map[string]string{
						DependencyAnnotation: "{\"name\":\"test-ping-source\",\"apiVersion\":\"sources.knative.dev/v1alpha1\"}",
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyFilter,
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
						DependencyAnnotation: "{\"kind\":\"PingSource\",\"apiVersion\":\"sources.knative.dev/v1alpha1\"}",
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyFilter,
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
						DependencyAnnotation: "{\"kind\":\"CronJobSource\",\"name\":\"test-cronjob-source\"}",
					}},
				Spec: TriggerSpec{
					Broker:     "test_broker",
					Filter:     validEmptyFilter,
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
					Filter:     validEmptyFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths: []string{
					dependencyAnnotationPath + "." + "kind",
					dependencyAnnotationPath + "." + "name",
					dependencyAnnotationPath + "." + "apiVersion"},
				Message: "missing field(s)",
			},
		}, {
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
					Filter:     validEmptyFilter,
					Subscriber: validSubscriber,
				}},
			want: &apis.FieldError{
				Paths:   []string{injectionAnnotationPath},
				Message: "The provided injection annotation value can only be \"enabled\" or \"disabled\", not \"wut\"",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.t.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("Trigger.Validate (-want, +got) = %v", diff)
			}
		})
	}
}

func TestTriggerSpecValidation(t *testing.T) {
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
			Filter:     validAttributesFilter,
			Subscriber: validSubscriber,
		},
		want: func() *apis.FieldError {
			fe := apis.ErrMissingField("broker")
			return fe
		}(),
	}, {
		name: "missing attributes keys, match all",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: &TriggerFilter{
				Attributes: TriggerFilterAttributes{},
			},
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "invalid attribute name, start with number",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: &TriggerFilter{
				Attributes: TriggerFilterAttributes{
					"0invalid": "my-value",
				},
			},
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{
			Message: "Invalid attribute name: \"0invalid\"",
			Paths:   []string{"filter.attributes"},
		},
	}, {
		name: "invalid attribute name, capital letters",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: &TriggerFilter{
				Attributes: TriggerFilterAttributes{
					"invALID": "my-value",
				},
			},
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{
			Message: "Invalid attribute name: \"invALID\"",
			Paths:   []string{"filter.attributes"},
		},
	}, {
		name: "missing subscriber",
		ts: &TriggerSpec{
			Broker: "test_broker",
			Filter: validAttributesFilter,
		},
		want: apis.ErrGeneric("expected at least one, got none", "subscriber.ref", "subscriber.uri"),
	}, {
		name: "missing subscriber.ref.name",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validAttributesFilter,
			Subscriber: invalidSubscriber,
		},
		want: apis.ErrMissingField("subscriber.ref.name"),
	}, {
		name: "missing broker",
		ts: &TriggerSpec{
			Broker:     "",
			Filter:     validAttributesFilter,
			Subscriber: validSubscriber,
		},
		want: apis.ErrMissingField("broker"),
	}, {
		name: "valid empty filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validEmptyFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "valid SourceAndType filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validAttributesFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}, {
		name: "valid Attributes filter",
		ts: &TriggerSpec{
			Broker:     "test_broker",
			Filter:     validAttributesFilter,
			Subscriber: validSubscriber,
		},
		want: &apis.FieldError{},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.Validate(context.TODO())
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate TriggerSpec (-want, +got) = %v", test.name, diff)
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
				Broker: "broker",
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker: "broker",
			},
		},
		want: nil,
	}, {
		name: "new nil is ok",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker: "broker",
			},
		},
		original: nil,
		want:     nil,
	}, {
		name: "good (filter change)",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker: "broker",
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker: "broker",
				Filter: validAttributesFilter,
			},
		},
		want: nil,
	}, {
		name: "bad (broker change)",
		current: &Trigger{
			Spec: TriggerSpec{
				Broker: "broker",
			},
		},
		original: &Trigger{
			Spec: TriggerSpec{
				Broker: "original_broker",
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
			got := test.current.CheckImmutableFields(context.TODO(), test.original)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("CheckImmutableFields (-want, +got) = %v", diff)
			}
		})
	}
}
