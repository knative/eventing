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

	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					triggerConditionBroker,
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "multiple conditions, condition false",
		ts: &TriggerStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					triggerConditionBroker,
					triggerConditionSubscribed,
				},
			},
		},
		condQuery: TriggerConditionSubscribed,
		want:      &triggerConditionSubscribed,
	}, {
		name: "unknown condition",
		ts: &TriggerStatus{
			Status: duckv1beta1.Status{
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
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
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionSubscribed,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &TriggerStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   TriggerConditionBroker,
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
		brokerStatus                *BrokerStatus
		markKubernetesServiceExists bool
		markVirtualServiceExists    bool
		subscriptionOwned           bool
		subscriptionStatus          *SubscriptionStatus
		wantReady                   bool
	}{{
		name:                        "all happy",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		wantReady:                   true,
	}, {
		name:                        "broker sad",
		brokerStatus:                TestHelper.NotReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		wantReady:                   false,
	}, {
		name:                        "subscribed sad",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           true,
		subscriptionStatus:          TestHelper.NotReadySubscriptionStatus(),
		wantReady:                   false,
	}, {
		name:                        "subscription not owned",
		brokerStatus:                TestHelper.ReadyBrokerStatus(),
		markKubernetesServiceExists: true,
		markVirtualServiceExists:    true,
		subscriptionOwned:           false,
		subscriptionStatus:          TestHelper.ReadySubscriptionStatus(),
		wantReady:                   false,
	}, {
		name:                        "all sad",
		brokerStatus:                TestHelper.NotReadyBrokerStatus(),
		markKubernetesServiceExists: false,
		markVirtualServiceExists:    false,
		subscriptionOwned:           false,
		subscriptionStatus:          TestHelper.NotReadySubscriptionStatus(),
		wantReady:                   false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ts := &TriggerStatus{}
			if test.brokerStatus != nil {
				ts.PropagateBrokerStatus(test.brokerStatus)
			}
			if !test.subscriptionOwned {
				ts.MarkSubscriptionNotOwned(&Subscription{})
			} else if test.subscriptionStatus != nil {
				ts.PropagateSubscriptionStatus(test.subscriptionStatus)
			}
			got := ts.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestTriggerAnnotateUserInfo(t *testing.T) {
	const (
		u1 = "oveja@knative.dev"
		u2 = "cabra@knative.dev"
		u3 = "vaca@knative.dev"
	)

	withUserAnns := func(creator, updater string, t *Trigger) *Trigger {
		a := t.GetAnnotations()
		if a == nil {
			a = map[string]string{}
			defer t.SetAnnotations(a)
		}

		a[eventing.CreatorAnnotation] = creator
		a[eventing.UpdaterAnnotation] = updater

		return t
	}

	tests := []struct {
		name       string
		user       string
		this       *Trigger
		prev       *Trigger
		wantedAnns map[string]string
	}{
		{
			name: "create new trigger",
			user: u1,
			this: &Trigger{},
			prev: nil,
			wantedAnns: map[string]string{
				eventing.CreatorAnnotation: u1,
				eventing.UpdaterAnnotation: u1,
			},
		}, {
			name:       "update trigger which has no annotations without diff",
			user:       u1,
			this:       &Trigger{Spec: TriggerSpec{Broker: defaultBroker, Filter: defaultTriggerFilter}},
			prev:       &Trigger{Spec: TriggerSpec{Broker: defaultBroker, Filter: defaultTriggerFilter}},
			wantedAnns: map[string]string{},
		}, {
			name: "update trigger which has annotations without diff",
			user: u2,
			this: withUserAnns(u1, u1, &Trigger{Spec: TriggerSpec{Broker: defaultBroker, Filter: defaultTriggerFilter}}),
			prev: withUserAnns(u1, u1, &Trigger{Spec: TriggerSpec{Broker: defaultBroker, Filter: defaultTriggerFilter}}),
			wantedAnns: map[string]string{
				eventing.CreatorAnnotation: u1,
				eventing.UpdaterAnnotation: u1,
			},
		}, {
			name: "update trigger which has no annotations with diff",
			user: u2,
			this: &Trigger{Spec: TriggerSpec{Broker: defaultBroker}},
			prev: &Trigger{Spec: TriggerSpec{Broker: otherBroker}},
			wantedAnns: map[string]string{
				eventing.UpdaterAnnotation: u2,
			},
		}, {
			name: "update trigger which has annotations with diff",
			user: u3,
			this: withUserAnns(u1, u2, &Trigger{Spec: TriggerSpec{Broker: otherBroker}}),
			prev: withUserAnns(u1, u2, &Trigger{Spec: TriggerSpec{Broker: defaultBroker}}),
			wantedAnns: map[string]string{
				eventing.CreatorAnnotation: u1,
				eventing.UpdaterAnnotation: u3,
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ctx := apis.WithUserInfo(context.Background(), &authv1.UserInfo{
				Username: test.user,
			})
			if test.prev != nil {
				ctx = apis.WithinUpdate(ctx, test.prev)
			}
			test.this.SetDefaults(ctx)

			if got, want := test.this.GetAnnotations(), test.wantedAnns; !cmp.Equal(got, want) {
				t.Errorf("Annotations = %v, want: %v, diff (-got, +want): %s", got, want, cmp.Diff(got, want))
			}
		})
	}
}
