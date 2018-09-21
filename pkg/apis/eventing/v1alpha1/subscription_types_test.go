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
		cs        *SubscriptionStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name: "single condition",
		cs: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &subscriptionConditionReady,
	}, {
		name: "multiple conditions",
		cs: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
				subscriptionConditionReferencesResolved,
			},
		},
		condQuery: SubscriptionConditionReferencesResolved,
		want:      &subscriptionConditionReferencesResolved,
	}, {
		name: "multiple conditions, condition true",
		cs: &SubscriptionStatus{
			Conditions: []duckv1alpha1.Condition{
				subscriptionConditionReady,
				subscriptionConditionFromReady,
			},
		},
		condQuery: SubscriptionConditionFromReady,
		want:      &subscriptionConditionFromReady,
	}, {
		name: "unknown condition",
		cs: &SubscriptionStatus{
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
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestSubscriptionSetConditions(t *testing.T) {
	c := &Subscription{
		Status: SubscriptionStatus{},
	}
	want := duckv1alpha1.Conditions{subscriptionConditionReady}
	c.Status.SetConditions(want)
	got := c.Status.GetConditions()
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected conditions (-want, +got) = %v", diff)
	}
}

func TestSubscriptionGetSpecJSON(t *testing.T) {
	targetURI := "http://example.com"
	c := &Subscription{
		Spec: SubscriptionSpec{
			From: corev1.ObjectReference{
				Name: "foo",
			},
			Call: &Callable{
				TargetURI: &targetURI,
			},
			Result: &ResultStrategy{
				Target: &corev1.ObjectReference{
					Name: "result",
				},
			},
		},
	}

	want := `{"from":{"name":"foo"},"call":{"targetURI":"http://example.com"},"result":{"target":{"name":"result"}}}`
	got, err := c.GetSpecJSON()
	if err != nil {
		t.Fatalf("unexpected spec JSON error: %v", err)
	}

	if diff := cmp.Diff(want, string(got)); diff != "" {
		t.Errorf("unexpected spec JSON (-want, +got) = %v", diff)
	}
}
