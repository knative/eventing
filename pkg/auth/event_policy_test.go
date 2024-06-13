package auth

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/strings/slices"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	eventpolicyinformerfake "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventpolicy/fake"
	reconcilertesting "knative.dev/pkg/reconciler/testing"
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
