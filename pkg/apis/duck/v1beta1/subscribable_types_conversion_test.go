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
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"

	eventingv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestSubscribableTypeConversionBadType(t *testing.T) {
	good, bad := &Subscribable{}, &pkgduckv1.Addressable{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1beta1 -> v1 -> v1beta1
func TestSubscribableTypeConversion(t *testing.T) {
	versions := []apis.Convertible{&eventingv1.Subscribable{}}

	linear := BackoffPolicyLinear

	tests := []struct {
		name string
		in   *Subscribable
	}{{
		name: "min configuration",
		in: &Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   SubscribableSpec{},
			Status: SubscribableStatus{},
		},
	}, {
		name: "full configuration",
		in: &Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: SubscribableSpec{
				Subscribers: []SubscriberSpec{
					{
						UID:           "uid-1",
						Generation:    7,
						SubscriberURI: apis.HTTP("subscriber.example.com"),
						ReplyURI:      apis.HTTP("reply.example.com"),
						Delivery: &DeliverySpec{
							DeadLetterSink: &pkgduckv1.Destination{
								Ref: &pkgduckv1.KReference{
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
			Status: SubscribableStatus{
				Subscribers: []SubscriberStatus{
					{
						UID:                "status-uid-1",
						ObservedGeneration: 99,
						Ready:              corev1.ConditionTrue,
						Message:            "msg",
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
				got := &Subscribable{}
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

// Test v1 -> v1beta1 -> v1
func TestSubscribableTypeConversionWithV1(t *testing.T) {
	versions := []apis.Convertible{&Subscribable{}}

	linear := eventingv1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *eventingv1.Subscribable
	}{{
		name: "min",
		in: &eventingv1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   eventingv1.SubscribableSpec{},
			Status: eventingv1.SubscribableStatus{},
		},
	}, {
		name: "full configuration",
		in: &eventingv1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: eventingv1.SubscribableSpec{
				Subscribers: []eventingv1.SubscriberSpec{
					{
						UID:           "uid-1",
						Generation:    7,
						SubscriberURI: apis.HTTP("subscriber.example.com"),
						ReplyURI:      apis.HTTP("reply.example.com"),
						Delivery: &eventingv1.DeliverySpec{
							DeadLetterSink: &pkgduckv1.Destination{
								Ref: &pkgduckv1.KReference{
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
			Status: eventingv1.SubscribableStatus{
				Subscribers: []eventingv1.SubscriberStatus{
					{
						UID:                "status-uid-1",
						ObservedGeneration: 99,
						Ready:              corev1.ConditionTrue,
						Message:            "msg",
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
				got := &eventingv1.Subscribable{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
