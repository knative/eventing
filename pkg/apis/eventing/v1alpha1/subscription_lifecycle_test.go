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
	"context"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing"
	"github.com/knative/pkg/apis"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	authv1 "k8s.io/api/authentication/v1"
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

var subscriptionConditionChannelReady = duckv1alpha1.Condition{
	Type:   SubscriptionConditionChannelReady,
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					subscriptionConditionReady,
				},
			},
		},
		condQuery: duckv1alpha1.ConditionReady,
		want:      &subscriptionConditionReady,
	}, {
		name: "multiple conditions",
		ss: &SubscriptionStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{
					subscriptionConditionReady,
					subscriptionConditionReferencesResolved,
				},
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   SubscriptionConditionChannelReady,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &SubscriptionStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
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
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
					Type:   SubscriptionConditionReferencesResolved,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &SubscriptionStatus{
			Status: duckv1alpha1.Status{
				Conditions: []duckv1alpha1.Condition{{
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
		name             string
		markResolved     bool
		markChannelReady bool
		wantReady        bool
	}{{
		name:             "all happy",
		markResolved:     true,
		markChannelReady: true,
		wantReady:        true,
	}, {
		name:             "one sad",
		markResolved:     false,
		markChannelReady: true,
		wantReady:        false,
	}, {
		name:             "other sad",
		markResolved:     true,
		markChannelReady: false,
		wantReady:        false,
	}, {
		name:             "both sad",
		markResolved:     false,
		markChannelReady: false,
		wantReady:        false,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ss := &SubscriptionStatus{}
			if test.markResolved {
				ss.MarkReferencesResolved()
			}
			if test.markChannelReady {
				ss.MarkChannelReady()
			}
			got := ss.IsReady()
			if test.wantReady != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantReady, got)
			}
		})
	}
}

func TestSubscriptionAnnotateUserInfo(t *testing.T) {
	const (
		u1 = "oveja@knative.dev"
		u2 = "cabra@knative.dev"
		u3 = "vaca@knative.dev"
	)

	withUserAnns := func(creator, updater string, s *Subscription) *Subscription {
		a := s.GetAnnotations()
		if a == nil {
			a = map[string]string{}
			defer s.SetAnnotations(a)
		}

		a[eventing.CreatorAnnotation] = creator
		a[eventing.UpdaterAnnotation] = updater

		return s
	}

	tests := []struct {
		name       string
		user       string
		this       *Subscription
		prev       *Subscription
		wantedAnns map[string]string
	}{{
		"create new subscription",
		u1,
		&Subscription{},
		nil,
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u1,
		},
	}, {
		"update subscription which has no annotations without diff",
		u1,
		&Subscription{},
		&Subscription{},
		map[string]string{},
	}, {
		"update subscription which has annotations without diff",
		u2,
		withUserAnns(u1, u1, &Subscription{}),
		withUserAnns(u1, u1, &Subscription{}),
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u1,
		},
	}, {
		"update subscription which has no annotations with diff",
		u2,
		&Subscription{Spec: SubscriptionSpec{DeprecatedGeneration: 1}},
		&Subscription{},
		map[string]string{
			eventing.UpdaterAnnotation: u2,
		}}, {
		"update subscription which has annotations with diff",
		u3,
		withUserAnns(u1, u2, &Subscription{Spec: SubscriptionSpec{DeprecatedGeneration: 1}}),
		withUserAnns(u1, u2, &Subscription{}),
		map[string]string{
			eventing.CreatorAnnotation: u1,
			eventing.UpdaterAnnotation: u3,
		},
	}}

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
