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

	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	triggerConditionReady = apis.Condition{
		Type:   TriggerConditionReady,
		Status: corev1.ConditionTrue,
	}

	triggerConditionBroker = apis.Condition{
		Type:   TriggerConditionBroker,
		Status: corev1.ConditionTrue,
	}

	triggerConditionDependency = apis.Condition{
		Type:   TriggerConditionDependency,
		Status: corev1.ConditionTrue,
	}

	triggerConditionSubscribed = apis.Condition{
		Type:   TriggerConditionSubscribed,
		Status: corev1.ConditionFalse,
	}
)

func TestTriggerGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ts        *TriggerStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					triggerConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &triggerConditionReady,
	}, {
		name: "multiple conditions",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					triggerConditionBroker,
					triggerConditionSubscribed,
					triggerConditionDependency,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "multiple conditions, condition false",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					triggerConditionBroker,
					triggerConditionSubscribed,
					triggerConditionDependency,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "unknown condition",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
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
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionDependency,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionUnknown,
				},
				},
			},
		},
	}, {
		name: "one false",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
					Status: corev1.ConditionFalse,
				}, {
					Type:   TriggerConditionDependency,
					Status: corev1.ConditionUnknown,
				},
					{
						Type:   TriggerConditionReady,
						Status: corev1.ConditionUnknown,
					}, {
						Type:   TriggerConditionSubscribed,
						Status: corev1.ConditionUnknown,
					},
				},
			},
		},
	}, {
		name: "one true",
		ts: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionDependency,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionTrue,
				},
				},
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
		brokerStatus                *BrokerStatus
		markKubernetesServiceExists bool
		markVirtualServiceExists    bool
		subscriptionOwned           bool
		subscriptionStatus          *messagingv1alpha1.SubscriptionStatus
		dependencyAnnotationExists  bool
		dependencyStatusReady       bool
		wantReady                   bool
	}{{
		name:                        "all happy",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		dependencyAnnotationExists:  false,
		wantReady:                   true,
	}, {
		name:                        "broker sad",
		brokerStatus:                TestHelper.NotReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		dependencyAnnotationExists:  false,
		wantReady:                   false,
	}, {
		name:                        "subscribed sad",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.NotReadySubscriptionStatus(),
		dependencyAnnotationExists:  false,
		wantReady:                   false,
	}, {
		name:                        "subscription not owned",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           false,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		dependencyAnnotationExists:  false,
		wantReady:                   false,
	}, {
		name:                        "dependency not ready",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		dependencyAnnotationExists:  true,
		dependencyStatusReady:       false,
		wantReady:                   false,
	},
		{
			name:                        "all sad",
			brokerStatus:                TestHelper.NotReadyBrokerStatus(),
			markKubernetesServiceExists: false,
			markVirtualServiceExists:    false,
			subscriptionOwned:           false,
			subscriptionStatus:          TestHelper.NotReadySubscriptionStatus(),
			dependencyAnnotationExists:  true,
			dependencyStatusReady:       false,
			wantReady:                   false,
		}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &TriggerStatus{}
			if test.brokerStatus != nil {
				ts.PropagateBrokerStatus(test.brokerStatus)
			}
			if !test.subscriptionOwned {
				ts.MarkSubscriptionNotOwned(&messagingv1alpha1.Subscription{})
			} else if test.subscriptionStatus != nil {
				ts.PropagateSubscriptionStatus(test.subscriptionStatus)
			}
			if test.dependencyAnnotationExists && !test.dependencyStatusReady {
				ts.MarkDependencyFailed("Dependency is not ready", "Dependency is not ready")
			} else {
				ts.MarkDependencySucceeded()
			}
			got := ts.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
