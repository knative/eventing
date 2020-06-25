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
	"testing"

	"knative.dev/pkg/apis"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var subscriptionConditionReady = apis.Condition{
	Type:   SubscriptionConditionReady,
	Status: corev1.ConditionTrue,
}

var subscriptionConditionReferencesResolved = apis.Condition{
	Type:   SubscriptionConditionReferencesResolved,
	Status: corev1.ConditionFalse,
}

var subscriptionConditionChannelReady = apis.Condition{
	Type:   SubscriptionConditionChannelReady,
	Status: corev1.ConditionTrue,
}

func TestSubscriptionGetConditionSet(t *testing.T) {
	r := &Subscription{}

	if got, want := r.GetConditionSet().GetTopLevelConditionType(), apis.ConditionReady; got != want {
		t.Errorf("GetTopLevelCondition=%v, want=%v", got, want)
	}
}

func TestSubscriptionGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ss        *SubscriptionStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					subscriptionConditionReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &subscriptionConditionReady,
	}, {
		name: "multiple conditions",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					subscriptionConditionReady,
					subscriptionConditionReferencesResolved,
				},
			},
		},
		condQuery: SubscriptionConditionReferencesResolved,
		want:      &subscriptionConditionReferencesResolved,
	}, {
		name: "multiple conditions, condition true",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					subscriptionConditionReady,
					subscriptionConditionChannelReady,
				},
			},
		},
		condQuery: SubscriptionConditionChannelReady,
		want:      &subscriptionConditionChannelReady,
	}, {
		name: "unknown condition",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					subscriptionConditionReady,
					subscriptionConditionReferencesResolved,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.ss.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		ss   *SubscriptionStatus
		want *SubscriptionStatus
	}{{
		name: "empty",
		ss:   &SubscriptionStatus{},
		want: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SubscriptionConditionAddedToChannel,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionChannelReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionReferencesResolved,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SubscriptionConditionChannelReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SubscriptionConditionAddedToChannel,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionChannelReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   SubscriptionConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionReferencesResolved,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one true",
		ss: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SubscriptionConditionReferencesResolved,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &SubscriptionStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   SubscriptionConditionAddedToChannel,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionChannelReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   SubscriptionConditionReferencesResolved,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ss.InitializeConditions()
			if diff := cmp.Diff(test.want, test.ss, ignoreAllButTypeAndStatus); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionIsReady(t *testing.T) {
	tests := []struct {
		name               string
		markResolved       bool
		markChannelReady   bool
		wantReady          bool
		markAddedToChannel bool
	}{{
		name:               "all happy",
		markResolved:       true,
		markChannelReady:   true,
		markAddedToChannel: true,
		wantReady:          true,
	}, {
		name:               "one sad - markResolved",
		markResolved:       false,
		markChannelReady:   true,
		markAddedToChannel: true,
		wantReady:          false,
	}, {
		name:               "one sad - markChannelReady",
		markResolved:       true,
		markChannelReady:   false,
		markAddedToChannel: true,
		wantReady:          false,
	}, {
		name:               "one sad - markAddedToChannel",
		markResolved:       true,
		markChannelReady:   true,
		markAddedToChannel: false,
		wantReady:          false,
	}, {
		name:             "other sad",
		markResolved:     true,
		markChannelReady: false,
		wantReady:        false,
	}, {
		name:               "all sad",
		markResolved:       false,
		markChannelReady:   false,
		markAddedToChannel: false,
		wantReady:          false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ss := &SubscriptionStatus{}
			if test.markResolved {
				ss.MarkReferencesResolved()
				if !ss.AreReferencesResolved() {
					t.Errorf("References marked resolved, but not reflected in AreReferencesResolved")
				}
			}
			if test.markChannelReady {
				ss.MarkChannelReady()
			}
			if test.markAddedToChannel {
				ss.MarkAddedToChannel()
				if !ss.IsAddedToChannel() {
					t.Errorf("Channel added, but not reflected in IsAddedToChannel")
				}
			}
			got := ss.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
