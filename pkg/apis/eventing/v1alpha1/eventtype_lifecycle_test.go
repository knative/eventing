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
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var (
	trueValue  = true
	falseValue = false
)

var (
	eventTypeConditionReady = duckv1alpha1.Condition{
		Type:   EventTypeConditionReady,
		Status: corev1.ConditionTrue,
	}

	eventTypeConditionBrokerExists = duckv1alpha1.Condition{
		Type:   EventTypeConditionBrokerExists,
		Status: corev1.ConditionTrue,
	}

	eventTypeConditionBrokerReady = duckv1alpha1.Condition{
		Type:   EventTypeConditionBrokerReady,
		Status: corev1.ConditionTrue,
	}
)

func TestEventTypeGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ets       *EventTypeStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					eventTypeConditionReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &eventTypeConditionReady,
	}, {
		name: "broker exists condition",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					eventTypeConditionBrokerExists,
				},
			},
		},
		condQuery: EventTypeConditionBrokerExists,
		want:      &eventTypeConditionBrokerExists,
	}, {
		name: "multiple conditions, condition true",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					eventTypeConditionBrokerExists,
					eventTypeConditionBrokerReady,
				},
			},
		},
		condQuery: EventTypeConditionBrokerReady,
		want:      &eventTypeConditionBrokerReady,
	}, {
		name: "unknown condition",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					eventTypeConditionBrokerReady,
					eventTypeConditionReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ets.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestEventTypeInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ets  *EventTypeStatus
		want *EventTypeStatus
	}{{
		name: "empty",
		ets:  &EventTypeStatus{},
		want: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   EventTypeConditionBrokerExists,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionBrokerReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionReady,
					Status: corev1.ConditionUnknown,
				},
				},
			},
		},
	}, {
		name: "one false",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   EventTypeConditionBrokerExists,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   EventTypeConditionBrokerExists,
					Status: corev1.ConditionFalse,
				}, {
					Type:   EventTypeConditionBrokerReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		ets: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   EventTypeConditionBrokerReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &EventTypeStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   EventTypeConditionBrokerExists,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionBrokerReady,
					Status: corev1.ConditionTrue,
				}, {
					Type:   EventTypeConditionReady,
					Status: corev1.ConditionUnknown,
				}},
			},
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ets.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ets, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestEventTypeIsReady(t *testing.T) {
	tests := []struct {
		name             string
		markBrokerExists *bool
		markBrokerReady  *bool
		wantReady        bool
	}{{
		name:             "all happy",
		markBrokerExists: &trueValue,
		markBrokerReady:  &trueValue,
		wantReady:        true,
	}, {
		name:             "broker exist sad",
		markBrokerExists: &falseValue,
		markBrokerReady:  &trueValue,
		wantReady:        false,
	}, {
		name:             "broker ready sad",
		markBrokerExists: &trueValue,
		markBrokerReady:  &falseValue,
		wantReady:        false,
	}, {
		name:             "all sad",
		markBrokerExists: &falseValue,
		markBrokerReady:  &falseValue,
		wantReady:        false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ets := &EventTypeStatus{}
			if test.markBrokerExists != nil {
				if *test.markBrokerExists {
					ets.MarkBrokerExists()
				} else {
					ets.MarkBrokerDoesNotExist()
				}
			}
			if test.markBrokerReady != nil {
				if *test.markBrokerReady {
					ets.MarkBrokerReady()
				} else {
					ets.MarkBrokerNotReady()
				}
			}

			got := ets.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
