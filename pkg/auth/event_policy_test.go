/*
Copyright 2024 The Knative Authors

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

package auth

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	brokerinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1/broker/fake"
	eventpolicyinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/authstatus"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/ptr"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
)

func TestGetEventPoliciesForResource(t *testing.T) {

	tests := []struct {
		name               string
		resourceObjectMeta metav1.ObjectMeta
		existingPolicies   []v1alpha1.EventPolicy
		want               []string
		wantErr            bool
	}{
		{
			name: "No match",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Ref: &v1alpha1.EventPolicyToReference{
									Name:       "another-broker",
									Kind:       "Broker",
									APIVersion: "eventing.knative.dev/v1",
								},
							},
						},
					},
				},
			},
			want: []string{},
		}, {
			name: "No match (different namespace)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "another-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Ref: &v1alpha1.EventPolicyToReference{
									Name:       "my-broker",
									Kind:       "Broker",
									APIVersion: "eventing.knative.dev/v1",
								},
							},
						},
					},
				},
			},
			want: []string{},
		}, {
			name: "Match all (empty .spec.to)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: nil,
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		}, {
			name: "Direct reference to resource",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Ref: &v1alpha1.EventPolicyToReference{
									Name:       "my-broker",
									Kind:       "Broker",
									APIVersion: "eventing.knative.dev/v1",
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		}, {
			name: "Reference via selector to resource",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
				Labels: map[string]string{
					"key": "value",
				},
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"another-key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		}, {
			name: "Reference via selector to resource (multiple policies)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
				Labels: map[string]string{
					"key": "value",
				},
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Ref: &v1alpha1.EventPolicyToReference{
									Name:       "my-broker",
									Kind:       "Broker",
									APIVersion: "eventing.knative.dev/v1",
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy-2",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Another-Kind",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy-3",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "key",
												Operator: metav1.LabelSelectorOpExists,
											},
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
				"another-policy",
				"another-policy-3",
			},
		}, {
			name: "Reference via selector to resource (multiple policies - not all matching)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
				Labels: map[string]string{
					"key": "value",
				},
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"another-key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy-2",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Another-Kind",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		}, {
			name: "Match (ignore ref.APIVersion version)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Ref: &v1alpha1.EventPolicyToReference{
									Name:       "my-broker",
									Kind:       "Broker",
									APIVersion: "eventing.knative.dev/v12345",
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		}, {
			name: "Match (ignore selector.APIVersion version)",
			resourceObjectMeta: metav1.ObjectMeta{
				Name:      "my-broker",
				Namespace: "my-namespace",
				Labels: map[string]string{
					"key": "value",
				},
			},
			existingPolicies: []v1alpha1.EventPolicy{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-policy-1",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v12345",
									},
								},
							},
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "another-policy",
						Namespace: "my-namespace",
					},
					Spec: v1alpha1.EventPolicySpec{
						To: []v1alpha1.EventPolicySpecTo{
							{
								Selector: &v1alpha1.EventPolicySelector{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"another-key": "value",
										},
									},
									TypeMeta: &metav1.TypeMeta{
										Kind:       "Broker",
										APIVersion: "eventing.knative.dev/v1",
									},
								},
							},
						},
					},
				},
			},
			want: []string{
				"my-policy-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := reconcilertesting.SetupFakeContext(t)

			for i := range tt.existingPolicies {
				err := eventpolicyinformerfake.Get(ctx).Informer().GetStore().Add(&tt.existingPolicies[i])
				if err != nil {
					t.Fatalf("error adding policies: %v", err)
				}
			}

			brokerGVK := eventingv1.SchemeGroupVersion.WithKind("Broker")
			got, err := GetEventPoliciesForResource(eventpolicyinformerfake.Get(ctx).Lister(), brokerGVK, tt.resourceObjectMeta)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEventPoliciesForResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotNames := make([]string, 0, len(got))
			for _, p := range got {
				gotNames = append(gotNames, p.Name)
			}

			if len(gotNames) != len(tt.want) {
				t.Errorf("GetEventPoliciesForResource() len(got) = %d, want %d", len(gotNames), len(tt.want))
			}

			for _, wantName := range tt.want {
				if !slices.Contains(gotNames, wantName) {
					t.Errorf("GetEventPoliciesForResource() got = %q, want %q. Missing %q", strings.Join(gotNames, ","), strings.Join(tt.want, ","), wantName)
				}
			}
		})
	}
}

func TestResolveSubjects(t *testing.T) {
	namespace := "my-ns"

	tests := []struct {
		name    string
		froms   []v1alpha1.EventPolicySpecFrom
		objects []runtime.Object
		want    []string
		wantErr bool
	}{
		{
			name: "simple",
			froms: []v1alpha1.EventPolicySpecFrom{
				{
					Ref: &v1alpha1.EventPolicyFromReference{
						APIVersion: "sources.knative.dev/v1",
						Kind:       "ApiServerSource",
						Name:       "my-source",
						Namespace:  namespace,
					},
				}, {
					Sub: ptr.String("system:serviceaccount:my-ns:my-app"),
				}, {
					Sub: ptr.String("system:serviceaccount:my-ns:my-app-2"),
				},
			},
			objects: []runtime.Object{
				&sourcesv1.ApiServerSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-source",
						Namespace: namespace,
					},
					Status: sourcesv1.ApiServerSourceStatus{
						SourceStatus: duckv1.SourceStatus{
							Auth: &duckv1.AuthStatus{
								ServiceAccountName: ptr.String("my-apiserversource-oidc-sa"),
							},
						},
					},
				},
				&eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: namespace,
					},
					Status: eventingv1.BrokerStatus{},
				},
			},
			want: []string{
				"system:serviceaccount:my-ns:my-apiserversource-oidc-sa",
				"system:serviceaccount:my-ns:my-app",
				"system:serviceaccount:my-ns:my-app-2",
			},
		}, {
			name: "multiple references",
			froms: []v1alpha1.EventPolicySpecFrom{
				{
					Ref: &v1alpha1.EventPolicyFromReference{
						APIVersion: "sources.knative.dev/v1",
						Kind:       "ApiServerSource",
						Name:       "my-source",
						Namespace:  namespace,
					},
				}, {
					Ref: &v1alpha1.EventPolicyFromReference{
						APIVersion: "sources.knative.dev/v1",
						Kind:       "PingSource",
						Name:       "my-pingsource",
						Namespace:  namespace,
					},
				}, {
					Sub: ptr.String("system:serviceaccount:my-ns:my-app"),
				}, {
					Sub: ptr.String("system:serviceaccount:my-ns:my-app-2"),
				},
			},
			objects: []runtime.Object{
				&sourcesv1.ApiServerSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-source",
						Namespace: namespace,
					},
					Status: sourcesv1.ApiServerSourceStatus{
						SourceStatus: duckv1.SourceStatus{
							Auth: &duckv1.AuthStatus{
								ServiceAccountName: ptr.String("my-apiserversource-oidc-sa"),
							},
						},
					},
				},
				&sourcesv1.PingSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pingsource",
						Namespace: namespace,
					},
					Status: sourcesv1.PingSourceStatus{
						SourceStatus: duckv1.SourceStatus{
							Auth: &duckv1.AuthStatus{
								ServiceAccountName: ptr.String("my-pingsource-oidc-sa"),
							},
						},
					},
				},
				&eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: namespace,
					},
					Status: eventingv1.BrokerStatus{},
				},
			},
			want: []string{
				"system:serviceaccount:my-ns:my-apiserversource-oidc-sa",
				"system:serviceaccount:my-ns:my-pingsource-oidc-sa",
				"system:serviceaccount:my-ns:my-app",
				"system:serviceaccount:my-ns:my-app-2",
			},
		}, {
			name: "reference has not auth status",
			froms: []v1alpha1.EventPolicySpecFrom{
				{
					Ref: &v1alpha1.EventPolicyFromReference{
						APIVersion: "eventing.knative.dev/v1",
						Kind:       "Broker",
						Name:       "my-broker",
						Namespace:  namespace,
					},
				},
			},
			objects: []runtime.Object{
				&eventingv1.Broker{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: namespace,
					},
					Status: eventingv1.BrokerStatus{},
				},
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			ctx, _ := fakedynamicclient.With(context.Background(), scheme.Scheme, tt.objects...)
			ctx = authstatus.WithDuck(ctx)
			r := resolver.NewAuthenticatableResolverFromTracker(ctx, tracker.New(func(types.NamespacedName) {}, 0))

			ep := &v1alpha1.EventPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-policy",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.EventPolicySpec{
					From: tt.froms,
				},
			}

			got, gotErr := ResolveSubjects(r, ep)
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("ResolveSubjects() error = %v, wantErr %v", gotErr, tt.wantErr)
				return
			}

			if !cmp.Equal(got, tt.want) {
				t.Errorf("Unexpected object (-want, +got) =\n%s", cmp.Diff(got, tt.want))
			}
		})
	}
}

func TestSubjectContained(t *testing.T) {

	tests := []struct {
		name        string
		sub         string
		allowedSubs []string
		want        bool
	}{
		{
			name: "simple 1:1 match",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns:my-sa",
			},
			want: true,
		}, {
			name: "simple 1:n match",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns:another-sa",
				"system:serviceaccounts:my-ns:my-sa",
				"system:serviceaccounts:my-ns:yet-another-sa",
			},
			want: true,
		}, {
			name: "pattern match (all)",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"*",
			},
			want: true,
		}, {
			name: "pattern match (namespace)",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns:*",
			},
			want: true,
		}, {
			name: "pattern match (different namespace)",
			sub:  "system:serviceaccounts:my-ns-2:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns:*",
			},
			want: false,
		}, {
			name: "pattern match (namespace prefix)",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns*",
			},
			want: true,
		}, {
			name: "pattern match (namespace prefix 2)",
			sub:  "system:serviceaccounts:my-ns-2:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:my-ns*",
			},
			want: true,
		}, {
			name: "pattern match (middle)",
			sub:  "system:serviceaccounts:my-ns:my-sa",
			allowedSubs: []string{
				"system:serviceaccounts:*:my-sa",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubjectContained(tt.sub, tt.allowedSubs); got != tt.want {
				t.Errorf("SubjectContained(%q, '%v') = %v, want %v", tt.sub, tt.allowedSubs, got, tt.want)
			}
		})
	}
}

func TestGetApplyingResourcesOfEventPolicyForGK(t *testing.T) {
	tests := []struct {
		name              string
		eventPolicySpecTo []v1alpha1.EventPolicySpecTo
		gk                schema.GroupKind
		brokerObjects     []*eventingv1.Broker
		want              []string
		wantErr           bool
	}{
		{
			name: "Returns resource from direct reference",
			eventPolicySpecTo: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				},
			},
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Broker",
			},
			brokerObjects: []*eventingv1.Broker{}, //for a direct reference, we don't need the indexer later
			want: []string{
				"my-broker",
			},
		}, {
			name: "Ignores resources of other Group&Kind in direct reference",
			eventPolicySpecTo: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				}, {
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Another-Kind",
						Name:       "another-res",
					},
				},
			},
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Broker",
			},
			brokerObjects: []*eventingv1.Broker{},
			want: []string{
				"my-broker",
			},
		}, {
			name: "Returns object which match selector",
			eventPolicySpecTo: []v1alpha1.EventPolicySpecTo{
				{
					Selector: &v1alpha1.EventPolicySelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": "value",
							},
						},
						TypeMeta: &metav1.TypeMeta{
							APIVersion: eventingv1.SchemeGroupVersion.String(),
							Kind:       "Broker",
						},
					},
				},
			},
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Broker",
			},
			brokerObjects: []*eventingv1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: "my-ns",
						Labels: map[string]string{
							"key": "value",
						},
					},
				}, {
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-other-broker",
						Namespace: "my-ns",
						Labels: map[string]string{
							"other-key": "other-value",
						},
					},
				},
			},
			want: []string{
				"my-broker",
			},
		}, {
			name: "Checks on GKs on selector match",
			eventPolicySpecTo: []v1alpha1.EventPolicySpecTo{
				{
					Selector: &v1alpha1.EventPolicySelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": "value",
							},
						},
						TypeMeta: &metav1.TypeMeta{
							APIVersion: eventingv1.SchemeGroupVersion.String(),
							Kind:       "Broker",
						},
					},
				},
			},
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Other-Kind",
			},
			brokerObjects: []*eventingv1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: "my-ns",
						Labels: map[string]string{
							"key": "value",
						},
					},
				},
			},
			want: []string{},
		}, {
			name:              "Empty .spec.to matches everything in namespace",
			eventPolicySpecTo: nil,
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Broker",
			},
			brokerObjects: []*eventingv1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: "my-ns",
						Labels: map[string]string{
							"key": "value",
						},
					},
				},
			},
			want: []string{
				"my-broker",
			},
		}, {
			name: "Returns elements only once in slice",
			eventPolicySpecTo: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				},
				{
					Selector: &v1alpha1.EventPolicySelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": "value",
							},
						},
						TypeMeta: &metav1.TypeMeta{
							APIVersion: eventingv1.SchemeGroupVersion.String(),
							Kind:       "Broker",
						},
					},
				},
			},
			gk: schema.GroupKind{
				Group: "eventing.knative.dev",
				Kind:  "Broker",
			},
			brokerObjects: []*eventingv1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-broker",
						Namespace: "my-ns",
						Labels: map[string]string{
							"key": "value",
						},
					},
				},
			},
			want: []string{
				"my-broker",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := reconcilertesting.SetupFakeContext(t)

			brokerIndexer := brokerinformerfake.Get(ctx).Informer().GetIndexer()
			for _, b := range tt.brokerObjects {
				err := brokerIndexer.Add(b)
				if err != nil {
					t.Fatalf("could not add broker object to indexer: %v", err)
				}
			}

			eventPolicy := &v1alpha1.EventPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-policy",
					Namespace: "my-ns",
				},
				Spec: v1alpha1.EventPolicySpec{
					To: tt.eventPolicySpecTo,
				},
			}

			got, err := GetApplyingResourcesOfEventPolicyForGK(eventPolicy, tt.gk, brokerIndexer)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetApplyingResourcesOfEventPolicyForGK() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetApplyingResourcesOfEventPolicyForGK() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventPolicyEventHandler_AddAndDelete(t *testing.T) {
	eventPolicy := &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-policy",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.EventPolicySpec{
			To: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				},
			},
		},
	}

	gk := schema.GroupKind{
		Group: eventingv1.SchemeGroupVersion.Group,
		Kind:  "Broker",
	}

	wantCalls := []string{
		"my-broker",
	}

	calls := map[string]int{}
	callbackFn := func(key types.NamespacedName) {
		calls[key.Name]++
	}

	handler := EventPolicyEventHandler(nil, gk, callbackFn)
	handler.OnAdd(eventPolicy, false)

	if len(calls) != len(wantCalls) {
		t.Errorf("EventPolicyEventHandler() callback in ADD was called on wrong resources. Want to be called on %v, but was called on %v", wantCalls, calls)
	}
	for _, wantCallForResource := range wantCalls {
		num, ok := calls[wantCallForResource]
		if !ok {
			t.Errorf("EventPolicyEventHandler() callback in ADD was called on %s 0 times. Expected to be called only once", wantCallForResource)
		}

		if num != 1 {
			t.Errorf("EventPolicyEventHandler() callback in ADD was called on %s %d times. Expected to be called only once", wantCallForResource, num)
		}
	}

	// do the same for OnDelete
	calls = map[string]int{}
	handler.OnDelete(eventPolicy)

	if len(calls) != len(wantCalls) {
		t.Errorf("EventPolicyEventHandler() callback in DELETE was called on wrong resources. Want to be called on %v, but was called on %v", wantCalls, calls)
	}
	for _, wantCallForResource := range wantCalls {
		num, ok := calls[wantCallForResource]
		if !ok {
			t.Errorf("EventPolicyEventHandler() callback in DELETE was called on %s 0 times. Expected to be called only once", wantCallForResource)
		}

		if num != 1 {
			t.Errorf("EventPolicyEventHandler() callback in DELETE was called on %s %d times. Expected to be called only once", wantCallForResource, num)
		}
	}

}

func TestEventPolicyEventHandler_Update(t *testing.T) {
	oldEventPolicy := &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-policy",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.EventPolicySpec{
			To: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				},
			},
		},
	}
	newEventPolicy := &v1alpha1.EventPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-policy",
			Namespace: "my-ns",
		},
		Spec: v1alpha1.EventPolicySpec{
			To: []v1alpha1.EventPolicySpecTo{
				{
					Ref: &v1alpha1.EventPolicyToReference{
						APIVersion: eventingv1.SchemeGroupVersion.String(),
						Kind:       "Broker",
						Name:       "my-broker",
					},
				},
			},
		},
	}

	gk := schema.GroupKind{
		Group: eventingv1.SchemeGroupVersion.Group,
		Kind:  "Broker",
	}

	wantCalls := []string{
		"my-broker",
	}

	calls := map[string]int{}
	callbackFn := func(key types.NamespacedName) {
		calls[key.Name]++
	}

	handler := EventPolicyEventHandler(nil, gk, callbackFn)
	handler.OnUpdate(oldEventPolicy, newEventPolicy)

	if len(calls) != len(wantCalls) {
		t.Errorf("EventPolicyEventHandler() callback in UPDATE was called on wrong resources. Want to be called on %v, but was called on %v", wantCalls, calls)
	}
	for _, wantCallForResource := range wantCalls {
		num, ok := calls[wantCallForResource]
		if !ok {
			t.Errorf("EventPolicyEventHandler() callback in UPDATE was called on %s 0 times. Expected to be called only once", wantCallForResource)
		}

		if num != 1 {
			t.Errorf("EventPolicyEventHandler() callback in UPDATE was called on %s %d times. Expected to be called only once", wantCallForResource, num)
		}
	}
}
