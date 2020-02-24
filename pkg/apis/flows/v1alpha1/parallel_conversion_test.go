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

package v1alpha1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/flows/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestParallelConversionBadType(t *testing.T) {
	good, bad := &Parallel{}, &Parallel{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestParallelRoundTripV1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Parallel{}}

	tests := []struct {
		name string
		in   *Parallel
	}{{
		name: "min configuration",
		in: &Parallel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "par-name",
				Namespace:  "par-ns",
				Generation: 17,
			},
			Spec: ParallelSpec{
				Branches: []ParallelBranch{},
			},
		},
	}, {
		name: "full configuration",
		in: &Parallel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "par-name",
				Namespace:  "par-ns",
				Generation: 17,
			},
			Spec: ParallelSpec{
				Branches: []ParallelBranch{
					{
						Filter: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "f1Kind",
								Namespace:  "f1Namespace",
								Name:       "f1Name",
								APIVersion: "f1APIVersion",
							},
							URI: apis.HTTP("f1.example.com")},

						Subscriber: duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "s1Kind",
								Namespace:  "s1Namespace",
								Name:       "s1Name",
								APIVersion: "s1APIVersion",
							},
							URI: apis.HTTP("s1.example.com")},

						Reply: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "reply1Kind",
								Namespace:  "reply1Namespace",
								Name:       "reply1Name",
								APIVersion: "reply1APIVersion",
							},
							URI: apis.HTTP("reply1.example.com"),
						},
					},
					{
						Filter: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "f2Kind",
								Namespace:  "f2Namespace",
								Name:       "f2Name",
								APIVersion: "f2APIVersion",
							},
							URI: apis.HTTP("f2.example.com")},

						Subscriber: duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "s2Kind",
								Namespace:  "s2Namespace",
								Name:       "s2Name",
								APIVersion: "s2APIVersion",
							},
							URI: apis.HTTP("s2.example.com")},

						Reply: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "reply2Kind",
								Namespace:  "reply2Namespace",
								Name:       "reply2Name",
								APIVersion: "reply2APIVersion",
							},
							URI: apis.HTTP("reply2.example.com"),
						},
					},
				},
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: metav1.TypeMeta{
						Kind:       "channelKind",
						APIVersion: "channelAPIVersion",
					},
				},
				Reply: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "replyKind",
						Namespace:  "replyNamespace",
						Name:       "replyName",
						APIVersion: "replyAPIVersion",
					},
					URI: apis.HTTP("reply.example.com"),
				},
			},
			Status: ParallelStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				AddressStatus: duckv1.AddressStatus{
					Address: &duckv1.Addressable{
						URL: apis.HTTP("addressstatus.example.com"),
					},
				},
				IngressChannelStatus: ParallelChannelStatus{
					Channel: corev1.ObjectReference{
						Kind:       "i-channel-kind",
						APIVersion: "i-channel-apiversion",
						Name:       "i-channel-name",
						Namespace:  "i-channel-namespace",
					},
					ReadyCondition: apis.Condition{Message: "i1-msg"},
				},
				BranchStatuses: []ParallelBranchStatus{
					{
						FilterSubscriptionStatus: ParallelSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								Kind:       "f1-sub-kind",
								APIVersion: "f1-sub-apiversion",
								Name:       "f1-sub-name",
								Namespace:  "f1-sub-namespace",
							},
							ReadyCondition: apis.Condition{Message: "f1-msg"},
						},
						SubscriptionStatus: ParallelSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								Kind:       "s1-sub-kind",
								APIVersion: "s1-sub-apiversion",
								Name:       "s1-sub-name",
								Namespace:  "s1-sub-namespace",
							},
							ReadyCondition: apis.Condition{Message: "s1-msg"},
						},
						FilterChannelStatus: ParallelChannelStatus{
							Channel: corev1.ObjectReference{
								Kind:       "s1-channel-kind",
								APIVersion: "s1-channel-apiversion",
								Name:       "s1-channel-name",
								Namespace:  "s1-channel-namespace",
							},
							ReadyCondition: apis.Condition{Message: "c1-msg"},
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Parallel{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestParallelRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&Parallel{}}

	tests := []struct {
		name string
		in   *v1beta1.Parallel
	}{{
		name: "min configuration",
		in: &v1beta1.Parallel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "par-name",
				Namespace:  "par-ns",
				Generation: 17,
			},
			Spec: v1beta1.ParallelSpec{
				Branches: []v1beta1.ParallelBranch{},
			},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.Parallel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "par-name",
				Namespace:  "par-ns",
				Generation: 17,
			},
			Spec: v1beta1.ParallelSpec{
				Branches: []v1beta1.ParallelBranch{
					{
						Filter: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "f1Kind",
								Namespace:  "f1Namespace",
								Name:       "f1Name",
								APIVersion: "f1APIVersion",
							},
							URI: apis.HTTP("f1.example.com")},

						Subscriber: duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "s1Kind",
								Namespace:  "s1Namespace",
								Name:       "s1Name",
								APIVersion: "s1APIVersion",
							},
							URI: apis.HTTP("s1.example.com")},

						Reply: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "reply1Kind",
								Namespace:  "reply1Namespace",
								Name:       "reply1Name",
								APIVersion: "reply1APIVersion",
							},
							URI: apis.HTTP("reply1.example.com"),
						},
					},
					{
						Filter: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "f2Kind",
								Namespace:  "f2Namespace",
								Name:       "f2Name",
								APIVersion: "f2APIVersion",
							},
							URI: apis.HTTP("f2.example.com")},

						Subscriber: duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "s2Kind",
								Namespace:  "s2Namespace",
								Name:       "s2Name",
								APIVersion: "s2APIVersion",
							},
							URI: apis.HTTP("s2.example.com")},

						Reply: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "reply2Kind",
								Namespace:  "reply2Namespace",
								Name:       "reply2Name",
								APIVersion: "reply2APIVersion",
							},
							URI: apis.HTTP("reply2.example.com"),
						},
					},
				},
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
					TypeMeta: metav1.TypeMeta{
						Kind:       "channelKind",
						APIVersion: "channelAPIVersion",
					},
				},
				Reply: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "replyKind",
						Namespace:  "replyNamespace",
						Name:       "replyName",
						APIVersion: "replyAPIVersion",
					},
					URI: apis.HTTP("reply.example.com"),
				},
			},
			Status: v1beta1.ParallelStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				AddressStatus: duckv1.AddressStatus{
					Address: &duckv1.Addressable{
						URL: apis.HTTP("addressstatus.example.com"),
					},
				},
				IngressChannelStatus: v1beta1.ParallelChannelStatus{
					Channel: corev1.ObjectReference{
						Kind:       "i-channel-kind",
						APIVersion: "i-channel-apiversion",
						Name:       "i-channel-name",
						Namespace:  "i-channel-namespace",
					},
					ReadyCondition: apis.Condition{Message: "i1-msg"},
				},
				BranchStatuses: []v1beta1.ParallelBranchStatus{
					{
						FilterSubscriptionStatus: v1beta1.ParallelSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								Kind:       "f1-sub-kind",
								APIVersion: "f1-sub-apiversion",
								Name:       "f1-sub-name",
								Namespace:  "f1-sub-namespace",
							},
							ReadyCondition: apis.Condition{Message: "f1-msg"},
						},
						SubscriptionStatus: v1beta1.ParallelSubscriptionStatus{
							Subscription: corev1.ObjectReference{
								Kind:       "s1-sub-kind",
								APIVersion: "s1-sub-apiversion",
								Name:       "s1-sub-name",
								Namespace:  "s1-sub-namespace",
							},
							ReadyCondition: apis.Condition{Message: "s1-msg"},
						},
						FilterChannelStatus: v1beta1.ParallelChannelStatus{
							Channel: corev1.ObjectReference{
								Kind:       "s1-channel-kind",
								APIVersion: "s1-channel-apiversion",
								Name:       "s1-channel-name",
								Namespace:  "s1-channel-namespace",
							},
							ReadyCondition: apis.Condition{Message: "c1-msg"},
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				got := &v1beta1.Parallel{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
