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

package v1beta1

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestSubscriptionConversionBadType(t *testing.T) {
	good, bad := &Subscription{}, &Channel{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestBrokerConversionBadVersion(t *testing.T) {
	good, bad := &Subscription{}, &Subscription{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1beta1 -> v1 -> v1beta1
func TestSubscriptionConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1.Subscription{}}

	linear := duckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *Subscription
	}{{
		name: "min configuration",
		in: &Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscription-name",
				Namespace:  "subscription-ns",
				Generation: 17,
			},
			Spec: SubscriptionSpec{},
		},
	}, {
		name: "full configuration",
		in: &Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscription-name",
				Namespace:  "subscription-ns",
				Generation: 17,
			},
			Spec: SubscriptionSpec{
				Channel: corev1.ObjectReference{
					Kind:       "channelKind",
					Namespace:  "channelNamespace",
					Name:       "channelName",
					APIVersion: "channelAPIVersion",
				},
				Subscriber: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "subscriber-dest-kind",
						Namespace:  "subscriber-dest-ns",
						Name:       "subscriber-dest-name",
						APIVersion: "subscriber-dest-version",
					},
					URI: apis.HTTP("address"),
				},
				Reply: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "reply-dest-kind",
						Namespace:  "reply-dest-ns",
						Name:       "reply-dest-name",
						APIVersion: "reply-dest-version",
					},
					URI: apis.HTTP("address"),
				},
				Delivery: &duckv1beta1.DeliverySpec{
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
			Status: SubscriptionStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				PhysicalSubscription: SubscriptionStatusPhysicalSubscription{
					SubscriberURI:     apis.HTTP("subscriber.example.com"),
					ReplyURI:          apis.HTTP("reply.example.com"),
					DeadLetterSinkURI: apis.HTTP("dlc.example.com"),
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Error("ConvertTo() =", err)
				}
				got := &Subscription{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Error("ConvertFrom() =", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Error("roundtrip (-want, +got) =", diff)
				}
			})
		}
	}
}

// Test v1 -> v1beta1 -> v1
func TestBrokerConversionRoundTripV1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&Subscription{}}

	linear := eventingduckv1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1.Subscription
	}{{
		name: "min configuration",
		in: &v1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscription-name",
				Namespace:  "subscription-ns",
				Generation: 17,
			},
			Spec: v1.SubscriptionSpec{},
		},
	}, {
		name: "full configuration",
		in: &v1.Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscription-name",
				Namespace:  "subscription-ns",
				Generation: 17,
			},
			Spec: v1.SubscriptionSpec{
				Channel: corev1.ObjectReference{
					Kind:       "channelKind",
					Namespace:  "channelNamespace",
					Name:       "channelName",
					APIVersion: "channelAPIVersion",
				},
				Subscriber: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "subscriber-dest-kind",
						Namespace:  "subscriber-dest-ns",
						Name:       "subscriber-dest-name",
						APIVersion: "subscriber-dest-version",
					},
					URI: apis.HTTP("address"),
				},
				Reply: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "reply-dest-kind",
						Namespace:  "reply-dest-ns",
						Name:       "reply-dest-name",
						APIVersion: "reply-dest-version",
					},
					URI: apis.HTTP("address"),
				},
				Delivery: &eventingduckv1.DeliverySpec{
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
			Status: v1.SubscriptionStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				PhysicalSubscription: v1.SubscriptionStatusPhysicalSubscription{
					SubscriberURI:     apis.HTTP("subscriber.example.com"),
					ReplyURI:          apis.HTTP("reply.example.com"),
					DeadLetterSinkURI: apis.HTTP("dlc.example.com"),
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Error("ConvertFrom() =", err)
				}
				got := &v1.Subscription{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Error("ConvertTo() =", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Error("roundtrip (-want, +got) =", diff)
				}
			})
		}
	}
}
