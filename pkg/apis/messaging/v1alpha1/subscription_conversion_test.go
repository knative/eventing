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
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging/v1beta1"
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

func TestSubscriptionConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Subscription{}}

	linear := duckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *Subscription
	}{{
		name: "min configuration",
		in: &Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: SubscriptionSpec{},
		},
	}, {
		name: "full configuration",
		in: &Subscription{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: SubscriptionSpec{
				Channel: corev1.ObjectReference{
					Kind:       "channelKind",
					Namespace:  "channelNamespace",
					Name:       "channelName",
					APIVersion: "channelAPIVersion",
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
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Subscription{}
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
