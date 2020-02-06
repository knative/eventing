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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"testing"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestBrokerConversionBadType(t *testing.T) {
	good, bad := &Broker{}, &dummy{}

	if err := good.ConvertUp(context.Background(), bad); err == nil {
		t.Errorf("ConvertUp() = %#v, wanted error", bad)
	}

	if err := good.ConvertDown(context.Background(), bad); err == nil {
		t.Errorf("ConvertDown() = %#v, wanted error", good)
	}
}

func TestBrokerConversion(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.Trigger{}}

	linear := v1alpha1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *Broker
	}{{
		name: "simple configuration",
		in: &Broker{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "broker-name",
				Namespace:  "broker-ns",
				Generation: 17,
			},
			Spec: BrokerSpec{
				ChannelTemplate: &v1alpha1.ChannelTemplateSpec{
					TypeMeta: metav1.TypeMeta{
						Kind:       "channelKind",
						APIVersion: "channelAPIVersion",
					},
				},
				Config: &duckv1.KReference{
					Kind:       "configKind",
					Namespace:  "configNamespace",
					Name:       "configName",
					APIVersion: "configAPIVersion",
				},
				Delivery: &v1alpha1.DeliverySpec{
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
			Status: BrokerStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
				},
				Address: duckv1alpha1.Addressable{
					Addressable: duckv1beta1.Addressable{
						URL: apis.HTTP("address"),
					},
				},
				TriggerChannel:
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
				got := &Trigger{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				// Since on the way down, we lose the DeprecatedSourceAndType,
				// convert the in to equivalent out.
				fixed := fixDeprecated(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Since DeprecatedSourceAndType is lossy but semanctically equivalent
// if source,type are present and equivalent in the attributes map,
// fix that so diff works.
func fixDeprecated(in *Trigger) *Trigger {
	if in.Spec.Filter != nil && in.Spec.Filter.DeprecatedSourceAndType != nil {
		// attributes must be nil, can't have both Deprecated / Attributes
		attributes := TriggerFilterAttributes{}
		attributes["source"] = in.Spec.Filter.DeprecatedSourceAndType.Source
		attributes["type"] = in.Spec.Filter.DeprecatedSourceAndType.Type
		in.Spec.Filter.DeprecatedSourceAndType = nil
		in.Spec.Filter.Attributes = &attributes
	}
	return in
}
