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
	"knative.dev/pkg/apis"
	pkgduckv1 "knative.dev/pkg/apis/duck/v1"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/duck/v1beta1"
)

func TestSubscribableTypeConversionBadType(t *testing.T) {
	good, bad := &SubscribableType{}, &pkgduckv1.Addressable{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestSubscribableTypeConversionV1alphaV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Subscribable{}}

	linear := v1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *SubscribableType
	}{{
		name: "min configuration",
		in: &SubscribableType{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   SubscribableTypeSpec{},
			Status: SubscribableTypeStatus{},
		},
	}, {
		name: "full configuration",
		in: &SubscribableType{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: SubscribableTypeSpec{
				Subscribable: &Subscribable{
					Subscribers: []SubscriberSpec{
						{
							UID:               "uid-1",
							Generation:        7,
							SubscriberURI:     apis.HTTP("subscriber.example.com"),
							ReplyURI:          apis.HTTP("reply.example.com"),
							DeadLetterSinkURI: apis.HTTP("subscriber.dls.example.com"),
							Delivery: &v1beta1.DeliverySpec{
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
						{
							UID:               "uid-2",
							Generation:        8,
							SubscriberURI:     apis.HTTP("subscriber2.example.com"),
							ReplyURI:          apis.HTTP("reply2.example.com"),
							DeadLetterSinkURI: apis.HTTP("subscriber2.dls.example.com"),
							Delivery:          nil,
						},
					},
				},
			},
			Status: SubscribableTypeStatus{
				SubscribableStatus: &SubscribableStatus{
					Subscribers: []v1beta1.SubscriberStatus{
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
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &SubscribableType{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				fixed := fixSubscribableTypeDeprecated(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1alpha1 -> v1 -> v1alpha1
func TestSubscribableTypeConversionV1alphaV1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1.Subscribable{}}

	linear := v1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *SubscribableType
	}{{
		name: "min configuration",
		in: &SubscribableType{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   SubscribableTypeSpec{},
			Status: SubscribableTypeStatus{},
		},
	}, {
		name: "full configuration",
		in: &SubscribableType{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: SubscribableTypeSpec{
				Subscribable: &Subscribable{
					Subscribers: []SubscriberSpec{
						{
							UID:               "uid-1",
							Generation:        7,
							SubscriberURI:     apis.HTTP("subscriber.example.com"),
							ReplyURI:          apis.HTTP("reply.example.com"),
							DeadLetterSinkURI: apis.HTTP("subscriber.dls.example.com"),
							Deliveryv1: &v1.DeliverySpec{
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
						{
							UID:               "uid-2",
							Generation:        8,
							SubscriberURI:     apis.HTTP("subscriber2.example.com"),
							ReplyURI:          apis.HTTP("reply2.example.com"),
							DeadLetterSinkURI: apis.HTTP("subscriber2.dls.example.com"),
							Deliveryv1:        nil,
						},
					},
				},
			},
			Status: SubscribableTypeStatus{
				SubscribableStatus: &SubscribableStatus{
					Subscribersv1: []v1.SubscriberStatus{
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
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &SubscribableType{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				fixed := fixSubscribableTypeDeprecatedv1(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestSubscribableTypeConversionWithV1beta1V1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&SubscribableType{}}

	linear := v1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1beta1.Subscribable
	}{{
		name: "min",
		in: &v1beta1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   v1beta1.SubscribableSpec{},
			Status: v1beta1.SubscribableStatus{},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: v1beta1.SubscribableSpec{
				Subscribers: []v1beta1.SubscriberSpec{
					{
						UID:           "uid-1",
						Generation:    7,
						SubscriberURI: apis.HTTP("subscriber.example.com"),
						ReplyURI:      apis.HTTP("reply.example.com"),
						Delivery: &v1beta1.DeliverySpec{
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
			Status: v1beta1.SubscribableStatus{
				Subscribers: []v1beta1.SubscriberStatus{
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
				got := &v1beta1.Subscribable{}
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

// Test v1 -> v1alpha1 -> v1
func TestSubscribableTypeConversionWithV1V1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&SubscribableType{}}

	linear := v1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1.Subscribable
	}{{
		name: "min",
		in: &v1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec:   v1.SubscribableSpec{},
			Status: v1.SubscribableStatus{},
		},
	}, {
		name: "full configuration",
		in: &v1.Subscribable{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "subscribable-name",
				Namespace:  "subscribable-ns",
				Generation: 17,
			},
			Spec: v1.SubscribableSpec{
				Subscribers: []v1.SubscriberSpec{
					{
						UID:           "uid-1",
						Generation:    7,
						SubscriberURI: apis.HTTP("subscriber.example.com"),
						ReplyURI:      apis.HTTP("reply.example.com"),
						Delivery: &v1.DeliverySpec{
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
			Status: v1.SubscribableStatus{
				Subscribers: []v1.SubscriberStatus{
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
				got := &v1.Subscribable{}
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

func TestSubscribableTypeSpecConversionBadType(t *testing.T) {
	good, bad := &SubscribableTypeSpec{}, &SubscribableTypeSpec{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestSubscribableTypeStatusConversionBadType(t *testing.T) {
	good, bad := &SubscribableTypeStatus{}, &SubscribableTypeStatus{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestSubscriberSpecConversionBadType(t *testing.T) {
	good, bad := &SubscriberSpec{}, &SubscriberSpec{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Since v1beta1 to v1alpha1 is lossy.
func fixSubscribableTypeDeprecated(in *SubscribableType) *SubscribableType {
	if in.Spec.Subscribable != nil && len(in.Spec.Subscribable.Subscribers) > 0 {
		for i := range in.Spec.Subscribable.Subscribers {
			if in.Spec.Subscribable.Subscribers[i].Delivery == nil {
				in.Spec.Subscribable.Subscribers[i].Delivery = &v1beta1.DeliverySpec{
					DeadLetterSink: &pkgduckv1.Destination{
						URI: in.Spec.Subscribable.Subscribers[i].DeadLetterSinkURI,
					},
				}
			}
		}
	}

	return in
}

// Since v1 to v1alpha1 is lossy.
func fixSubscribableTypeDeprecatedv1(in *SubscribableType) *SubscribableType {
	if in.Spec.Subscribable != nil && len(in.Spec.Subscribable.Subscribers) > 0 {
		for i := range in.Spec.Subscribable.Subscribers {
			if in.Spec.Subscribable.Subscribers[i].Deliveryv1 == nil {
				in.Spec.Subscribable.Subscribers[i].Deliveryv1 = &v1.DeliverySpec{
					DeadLetterSink: &pkgduckv1.Destination{
						URI: in.Spec.Subscribable.Subscribers[i].DeadLetterSinkURI,
					},
				}
			}
		}
	}

	return in
}
