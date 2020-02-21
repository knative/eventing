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
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	v1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
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
	versions := []apis.Convertible{&v1beta1.Broker{}}

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
				ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
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
				TriggerChannel: &corev1.ObjectReference{
					Kind:            "k",
					Namespace:       "ns",
					Name:            "n",
					UID:             "uid",
					APIVersion:      "av",
					ResourceVersion: "rv",
					FieldPath:       "fp",
				},
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
				got := &Broker{}
				if err := got.ConvertDown(context.Background(), ver); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				// Since on the way down, we lose the DeprecatedSourceAndType,
				// convert the in to equivalent out.
				fixed := fixBrokerDeprecated(test.in)
				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Since v1beta1 to v1alpha1 is lossy but semantically equivalent,
// fix that so diff works.
func fixBrokerDeprecated(in *Broker) *Broker {
	in.Spec.ChannelTemplate = nil
	in.Status.TriggerChannel = nil
	return in
}
