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
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// TODO: Replace dummy some other Eventing object once they
// implement apis.Convertible
type dummy struct{}

func (*dummy) ConvertUp(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func (*dummy) ConvertDown(ctx context.Context, obj apis.Convertible) error {
	return errors.New("Won't go")
}

func TestDeliverySpecConversionBadType(t *testing.T) {
	good, bad := &DeliverySpec{}, &dummy{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertDown() = %#v, wanted error", good)
	}
}

func TestDeliverySpecConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.DeliverySpec{}}

	linear := BackoffPolicyLinear

	tests := []struct {
		name string
		in   *DeliverySpec
	}{{
		name: "nil",
		in:   nil,
	}, {
		name: "empty configuration",
		in:   &DeliverySpec{},
	}, {name: "full configuration",
		in: &DeliverySpec{
			DeadLetterSink: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Name: "aname",
				},
				URI: &apis.URL{
					Scheme: "http",
					Host:   "test-error-domain",
				},
			},
			Retry:         pointer.Int32Ptr(5),
			BackoffPolicy: &linear,
			BackoffDelay:  pointer.StringPtr("5s"),
		},
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertUp(context.Background(), ver); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}
				got := &DeliverySpec{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				if test.in == nil {
					if diff := cmp.Diff(&DeliverySpec{}, got); diff != "" {
						t.Errorf("nil roundtrip (-want, +got) = %v", diff)
					}
				} else if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

func TestDeliveryStatusConversionBadType(t *testing.T) {
	good, bad := &DeliveryStatus{}, &dummy{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertDown() = %#v, wanted error", good)
	}
}

func TestDeliveryStatusConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.DeliveryStatus{}}

	tests := []struct {
		name string
		in   *DeliveryStatus
	}{{
		name: "nil",
		in:   nil,
	}, {
		name: "empty configuration",
		in:   &DeliveryStatus{},
	}, {
		name: "full configuration",
		in: &DeliveryStatus{
			DeadLetterChannel: &corev1.ObjectReference{
				Name:       "aname",
				Namespace:  "anamespace",
				APIVersion: "anapiversion",
				Kind:       "akind",
			},
		},
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertUp(context.Background(), ver); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}
				got := &DeliveryStatus{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				if test.in == nil {
					if diff := cmp.Diff(&DeliveryStatus{}, got); diff != "" {
						t.Errorf("nil roundtrip (-want, +got) = %v", diff)
					}
				} else if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
