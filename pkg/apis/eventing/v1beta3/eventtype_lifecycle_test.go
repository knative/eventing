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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
)

var (
	trueValue  = true
	falseValue = false

	eventTypeConditionReady = apis.Condition{
		Type:   EventTypeConditionReady,
		Status: corev1.ConditionTrue,
	}

	eventTypeConditionBrokerExists = apis.Condition{
		Type:   EventTypeConditionReferenceExists,
		Status: corev1.ConditionTrue,
	}

	ignoreAllButTypeAndStatus = cmpopts.IgnoreFields(
		apis.Condition{},
		"LastTransitionTime", "Message", "Reason", "Severity")
)

func TestEventTypeGetConditionSet(t *testing.T) {
	r := &EventType{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestEventTypeGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ets       *EventTypeStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ets: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					eventTypeConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &eventTypeConditionReady,
	}, {
		name: "broker exists condition",
		ets: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					eventTypeConditionBrokerExists,
				},
			},
		},
		condQuery: EventTypeConditionReferenceExists,
		want:      &eventTypeConditionBrokerExists,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ets.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Error("unexpected condition (-want, +got) =", diff)
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   EventTypeConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionReferenceExists,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		ets: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   EventTypeConditionReferenceExists,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   EventTypeConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   EventTypeConditionReferenceExists,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ets.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ets, ignoreAllButTypeAndStatus); diff != "" {
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func TestEventTypeConditionStatus(t *testing.T) {
	tests := []struct {
		name                string
		markResourceExists  *bool
		brokerStatus        *eventingv1.BrokerStatus
		wantConditionStatus corev1.ConditionStatus
	}{{
		name:                "all happy",
		markResourceExists:  &trueValue,
		brokerStatus:        eventingv1.TestHelper.ReadyBrokerStatus(),
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "all happy, dls not configured",
		markResourceExists:  &trueValue,
		brokerStatus:        eventingv1.TestHelper.ReadyBrokerStatusWithoutDLS(),
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "broker exist sad",
		markResourceExists:  &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "all sad",
		markResourceExists:  &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ets := &EventTypeStatus{}
			if test.markResourceExists != nil {
				if *test.markResourceExists {
					ets.MarkReferenceExists()
				} else {
					ets.MarkReferenceDoesNotExist()
				}
			}

			got := ets.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v\n%+v", test.wantConditionStatus, got, ets)
			}
		})
	}
}
