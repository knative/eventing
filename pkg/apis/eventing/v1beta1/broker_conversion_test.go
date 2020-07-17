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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBrokerConversionBadType(t *testing.T) {
	good, bad := &Broker{}, &Trigger{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestBrokerConversionBadVersion(t *testing.T) {
	good, bad := &Broker{}, &Broker{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1beta1 -> v1 -> v1beta1
func TestBrokerConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1.Broker{}}

	linear := eventingduckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *Broker
	}{{
		name: "min configuration",
		in: &Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: BrokerSpec{},
		},
	}, {
		name: "full configuration",
		in: &Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: BrokerSpec{
				Config: &duckv1.KReference{
					Kind:       "config-kind",
					Namespace:  "config-ns",
					Name:       "config-name",
					APIVersion: "config-version",
				},
				Delivery: &eventingduckv1beta1.DeliverySpec{
					DeadLetterSink: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "dl-sink-kind",
							Namespace:  "dl-sink-ns",
							Name:       "dl-sink-name",
							APIVersion: "dl-sink-version",
						},
						URI: apis.HTTP("dl-sink.example.com"),
					},
					Retry:         pointer.Int32Ptr(5),
					BackoffPolicy: &linear,
					BackoffDelay:  pointer.StringPtr("5s"),
				},
			},
			Status: BrokerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
					Annotations: map[string]string{
						"channelAddress": "http://foo.bar.svc.cluster.local/",
					},
				},
				Address: duckv1.Addressable{
					URL: apis.HTTP("address"),
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
				got := &Broker{}
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
func TestBrokerConversionRoundTripV1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&Broker{}}

	linear := eventingduckv1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1.Broker
	}{{
		name: "min configuration",
		in: &v1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "par-name",
				Namespace:  "par-ns",
				Generation: 17,
			},
			Spec: v1.BrokerSpec{},
		},
	}, {
		name: "full configuration",
		in: &v1.Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: v1.BrokerSpec{
				Config: &duckv1.KReference{
					Kind:       "config-kind",
					Namespace:  "config-ns",
					Name:       "config-name",
					APIVersion: "config-version",
				},
				Delivery: &eventingduckv1.DeliverySpec{
					DeadLetterSink: &duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "dl-sink-kind",
							Namespace:  "dl-sink-ns",
							Name:       "dl-sink-name",
							APIVersion: "dl-sink-version",
						},
						URI: apis.HTTP("dl-sink.example.com"),
					},
					Retry:         pointer.Int32Ptr(5),
					BackoffPolicy: &linear,
					BackoffDelay:  pointer.StringPtr("5s"),
				},
			},
			Status: v1.BrokerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
					Annotations: map[string]string{
						"channelAddress": "http://foo.bar.svc.cluster.local/",
					},
				},
				Address: duckv1.Addressable{
					URL: apis.HTTP("address"),
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				got := &v1.Broker{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
