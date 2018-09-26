/*
Copyright 2018 The Knative Authors

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
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

var subscriptionConditionReady = duckv1alpha1.Condition{
	Type:   SubscriptionConditionReady,
	Status: corev1.ConditionTrue,
}

var subscriptionConditionReferencesResolved = duckv1alpha1.Condition{
	Type:   SubscriptionConditionReferencesResolved,
	Status: corev1.ConditionFalse,
}

var subscriptionConditionFromReady = duckv1alpha1.Condition{
	Type:   SubscriptionConditionFromReady,
	Status: corev1.ConditionTrue,
}

func TestSubscriptionGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		ss        *SubscriptionStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &subscriptionConditionReady,
	}, {
		name: "multiple conditions",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
				subscriptionConditionReferencesResolved,
			},
		},
		condQuery: SubscriptionConditionReferencesResolved,
		want:      &subscriptionConditionReferencesResolved,
	}, {
		name: "multiple conditions, condition true",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
				subscriptionConditionFromReady,
			},
		},
		condQuery: SubscriptionConditionFromReady,
		want:      &subscriptionConditionFromReady,
	}, {
		name: "unknown condition",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
				subscriptionConditionReferencesResolved,
			},
		},
		condQuery: duckv1alpha1.ConditionType("foo"),
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
			Conditions: []duckv1alpha1.Condition{{
				Type:   SubscriptionConditionFromReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   SubscriptionConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   SubscriptionConditionReferencesResolved,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one false",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SubscriptionConditionFromReady,
				Status: corev1.ConditionFalse,
			}},
		},
		want: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SubscriptionConditionFromReady,
				Status: corev1.ConditionFalse,
			}, {
				Type:   SubscriptionConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   SubscriptionConditionReferencesResolved,
				Status: corev1.ConditionUnknown,
			}},
		},
	}, {
		name: "one true",
		ss: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SubscriptionConditionReferencesResolved,
				Status: corev1.ConditionTrue,
			}},
		},
		want: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{{
				Type:   SubscriptionConditionFromReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   SubscriptionConditionReady,
				Status: corev1.ConditionUnknown,
			}, {
				Type:   SubscriptionConditionReferencesResolved,
				Status: corev1.ConditionTrue,
			}},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.ss.InitializeConditions()
			ignore := cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime")
			if diff := cmp.Diff(test.want, test.ss, ignore); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionIsReady(t *testing.T) {
	tests := []struct {
		name          string
		markResolved  bool
		markFromReady bool
		wantReady     bool
	}{{
		name:          "all happy",
		markResolved:  true,
		markFromReady: true,
		wantReady:     true,
	}, {
		name:          "one sad",
		markResolved:  false,
		markFromReady: true,
		wantReady:     false,
	}, {
		name:          "other sad",
		markResolved:  true,
		markFromReady: false,
		wantReady:     false,
	}, {
		name:          "both sad",
		markResolved:  false,
		markFromReady: false,
		wantReady:     false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ss := &SubscriptionStatus{}
			if test.markResolved {
				ss.MarkReferencesResolved()
			}
			if test.markFromReady {
				ss.MarkFromReady()
			}
			got := ss.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}
