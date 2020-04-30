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
	"k8s.io/utils/pointer"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

func TestInMemoryChannelConversionBadType(t *testing.T) {
	good, bad := &InMemoryChannel{}, &Subscription{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestInMemoryChannelConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.InMemoryChannel{}}

	linear := eventingduckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *InMemoryChannel
	}{{
		name: "min configuration",
		in: &InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "channel-name",
				Namespace:  "channel-ns",
				Generation: 17,
			},
			Spec: InMemoryChannelSpec{},
			Status: InMemoryChannelStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "channel-name",
				Namespace:  "channel-ns",
				Generation: 17,
			},
			Spec: InMemoryChannelSpec{
				Subscribable: &eventingduck.Subscribable{
					Subscribers: []eventingduck.SubscriberSpec{
						{
							UID:               "uid-1",
							Generation:        7,
							SubscriberURI:     apis.HTTP("subscriber.example.com"),
							ReplyURI:          apis.HTTP("reply.example.com"),
							DeadLetterSinkURI: apis.HTTP("subscriber.dls.example.com"),
							Delivery: &eventingduckv1beta1.DeliverySpec{
								DeadLetterSink: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Kind:       "dlKind",
										Namespace:  "dlNamespace",
										Name:       "dlName",
										APIVersion: "dlAPIVersion",
									},
									URI: apis.HTTP("subscriber.dls.example.com"),
								},
								Retry:         pointer.Int32Ptr(5),
								BackoffPolicy: &linear,
								BackoffDelay:  pointer.StringPtr("5s"),
							},
						},
					},
				},
				Delivery: &eventingduckv1beta1.DeliverySpec{
					DeadLetterSink: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "dlKind",
							Namespace:  "dlNamespace",
							Name:       "dlName",
							APIVersion: "dlAPIVersion",
						},
						URI: apis.HTTP("dls"),
					},
					Retry:         pointer.Int32Ptr(5),
					BackoffPolicy: &linear,
					BackoffDelay:  pointer.StringPtr("5s"),
				},
			},
			Status: InMemoryChannelStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: apis.HTTP("addressstatus.example.com"),
						},
						Hostname: "addressstatus.example.com",
					},
				},
				SubscribableTypeStatus: eventingduck.SubscribableTypeStatus{
					SubscribableStatus: &eventingduck.SubscribableStatus{
						Subscribers: []eventingduck.SubscriberStatus{
							{
								UID:                "status-uid-1",
								ObservedGeneration: 99,
								Ready:              corev1.ConditionTrue,
								Message:            "msg",
							},
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
				got := &InMemoryChannel{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				// Make sure the annotation specifies the correct duck.
				if test.in.Annotations == nil {
					test.in.Annotations = make(map[string]string)
				}
				test.in.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1alpha1"

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestInMemoryChannelConversionWithV1Beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&InMemoryChannel{}}

	linear := eventingduckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1beta1.InMemoryChannel
	}{{
		name: "min",
		in: &v1beta1.InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "channel-name",
				Namespace:  "channel-ns",
				Generation: 17,
			},
			Spec: v1beta1.InMemoryChannelSpec{},
			Status: v1beta1.InMemoryChannelStatus{
				ChannelableStatus: eventingduckv1beta1.ChannelableStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{},
					},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.InMemoryChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "channel-name",
				Namespace:  "channel-ns",
				Generation: 17,
			},
			Spec: v1beta1.InMemoryChannelSpec{
				ChannelableSpec: eventingduckv1beta1.ChannelableSpec{
					SubscribableSpec: eventingduckv1beta1.SubscribableSpec{
						Subscribers: []eventingduckv1beta1.SubscriberSpec{
							{
								UID:           "uid-1",
								Generation:    7,
								SubscriberURI: apis.HTTP("subscriber.example.com"),
								ReplyURI:      apis.HTTP("reply.example.com"),
								//							DeadLetterSinkURI: apis.HTTP("dlc.reply.example.com"),
								Delivery: &eventingduckv1beta1.DeliverySpec{
									DeadLetterSink: &duckv1.Destination{
										Ref: &duckv1.KReference{
											Kind:       "dlKind",
											Namespace:  "dlNamespace",
											Name:       "dlName",
											APIVersion: "dlAPIVersion",
										},
										URI: apis.HTTP("subscriber.dls.example.com"),
									},
									Retry:         pointer.Int32Ptr(5),
									BackoffPolicy: &linear,
									BackoffDelay:  pointer.StringPtr("5s"),
								},
							},
						},
					},
					Delivery: &eventingduckv1beta1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: &duckv1.KReference{
								Kind:       "dlKind",
								Namespace:  "dlNamespace",
								Name:       "dlName",
								APIVersion: "dlAPIVersion",
							},
							URI: apis.HTTP("dls"),
						},
						Retry:         pointer.Int32Ptr(5),
						BackoffPolicy: &linear,
						BackoffDelay:  pointer.StringPtr("5s"),
					},
				},
			},
			Status: v1beta1.InMemoryChannelStatus{
				ChannelableStatus: eventingduckv1beta1.ChannelableStatus{
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
					SubscribableStatus: eventingduckv1beta1.SubscribableStatus{
						Subscribers: []eventingduckv1beta1.SubscriberStatus{
							{
								UID:                "status-uid-1",
								ObservedGeneration: 99,
								Ready:              corev1.ConditionTrue,
								Message:            "msg",
							},
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
				if err := version.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &v1beta1.InMemoryChannel{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				// Make sure the annotation specifies the correct duck.
				if test.in.Annotations == nil {
					test.in.Annotations = make(map[string]string)
				}
				test.in.Annotations[messaging.SubscribableDuckVersionAnnotation] = "v1beta1"

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
