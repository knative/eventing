package auth

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	sourcesv1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/client/clientset/versioned/scheme"
	eventpolicyinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/ptr"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
	"reflect"
	"strings"
	"testing"
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
				},
			},
			want: []string{
				"my-policy-1",
				"another-policy",
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()

			ctx, dynamicClient := fakedynamicclient.With(ctx, scheme.Scheme, tt.objects...)

			got, err := ResolveSubjects(ctx, dynamicClient, tt.froms, namespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveSubjects() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResolveSubjects() got = %v, want %v", got, tt.want)
			}
		})
	}
}
