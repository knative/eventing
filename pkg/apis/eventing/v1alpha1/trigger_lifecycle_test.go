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
	triggerConditionReady = duckv1alpha1.Condition{
		Type:   TriggerConditionReady,
		Status: corev1.ConditionTrue,
	}

	triggerConditionBrokerExists = duckv1alpha1.Condition{
		Type:   TriggerConditionBrokerExists,
		Status: corev1.ConditionTrue,
	}

	triggerConditionSubscribed = duckv1alpha1.Condition{
		Type:   TriggerConditionSubscribed,
		Status: corev1.ConditionFalse,
	}
)

func TestTriggerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ts        *TriggerStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					triggerConditionReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &triggerConditionReady,
	}, {
		name: "multiple conditions",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					triggerConditionBrokerExists,
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "multiple conditions, condition false",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					triggerConditionBrokerExists,
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "unknown condition",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ts.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestTriggerInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ts   *TriggerStatus
		want *TriggerStatus
	}{{
		name: "empty",
		ts:   &TriggerStatus{},
		want: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   TriggerConditionBrokerExists,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   TriggerConditionBrokerExists,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   TriggerConditionBrokerExists,
					Status: corev1.ConditionFalse,
				}, {
					Type:   TriggerConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		ts: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   TriggerConditionBrokerExists,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ts.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ts, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestTriggerIsReady(t *testing.T) {
	tests := []struct {
		name                        string
		markBrokerExists            bool
		markKubernetesServiceExists bool
		markVirtualServiceExists    bool
		markSubscribed              bool
		wantReady                   bool
	}{{
		name:                        "all happy",
		markBrokerExists:            true,
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		markSubscribed:              true,
		wantReady:                   true,
	}, {
		name:                        "broker sad",
		markBrokerExists:            false,
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		markSubscribed:              true,
		wantReady:                   false,
	}, {
		name:                        "subscribed sad",
		markBrokerExists:            true,
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		markSubscribed:              false,
		wantReady:                   false,
	}, {
		name:                        "all sad",
		markBrokerExists:            false,
		markKubernetesServiceExists: false,
		markVirtualServiceExists:    false,
		markSubscribed:              false,
		wantReady:                   false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &TriggerStatus{}
			if test.markBrokerExists {
				ts.MarkBrokerExists()
			}
			if test.markSubscribed {
				ts.MarkSubscribed()
			}
			got := ts.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
