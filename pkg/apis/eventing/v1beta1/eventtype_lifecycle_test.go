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

package v1beta1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	corev1 "k8s.io/api/core/v1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	trueValue  = true
	falseValue = false

	eventTypeConditionReady = apis.Condition{
		Type:   EventTypeConditionReady,
		Status: corev1.ConditionTrue,
	}

	eventTypeConditionBrokerExists = apis.Condition{
		Type:   EventTypeConditionBrokerExists,
		Status: corev1.ConditionTrue,
	}

	eventTypeConditionBrokerReady = apis.Condition{
		Type:   EventTypeConditionBrokerReady,
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
		condQuery: EventTypeConditionBrokerExists,
		want:      &eventTypeConditionBrokerExists,
	}, {
		name: "multiple conditions, condition true",
		ets: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					eventTypeConditionBrokerReady,
					eventTypeConditionReady,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   EventTypeConditionBrokerExists,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   EventTypeConditionBrokerReady,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &EventTypeStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
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
				t.Error("unexpected conditions (-want, +got) =", diff)
			}
		})
	}
}

func TestEventTypeConditionStatus(t *testing.T) {
	tests := []struct {
		name                string
		markBrokerExists    *bool
		brokerStatus        *eventingv1.BrokerStatus
		wantConditionStatus corev1.ConditionStatus
	}{{
		name:                "all happy",
		markBrokerExists:    &trueValue,
		brokerStatus:        eventingv1.TestHelper.ReadyBrokerStatus(),
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "all happy, dls not configured",
		markBrokerExists:    &trueValue,
		brokerStatus:        eventingv1.TestHelper.ReadyBrokerStatusWithoutDLS(),
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "broker exist sad",
		markBrokerExists:    &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "broker ready sad",
		markBrokerExists:    &trueValue,
		brokerStatus:        eventingv1.TestHelper.FalseBrokerStatus(),
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "broker ready unknown",
		markBrokerExists:    &trueValue,
		brokerStatus:        eventingv1.TestHelper.UnknownBrokerStatus(),
		wantConditionStatus: corev1.ConditionUnknown,
	}, {
		name:                "all sad",
		markBrokerExists:    &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
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
			if test.brokerStatus != nil {
				ets.PropagateBrokerStatus(test.brokerStatus)
			}

			got := ets.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantConditionStatus, got)
			}
		})
	}
}
