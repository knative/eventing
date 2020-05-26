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

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
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
					Hostname: "address",
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
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				got := &Broker{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
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

func TestBrokerPropagateStatus(t *testing.T) {
	tests := []struct {
		name                string
		markBrokerExists    *bool
		brokerStatus        *BrokerStatus
		wantConditionStatus corev1.ConditionStatus
	}{{
		name:                "all happy",
		markBrokerExists:    &trueValue,
		brokerStatus:        TestHelper.ReadyBrokerStatus(),
		wantConditionStatus: corev1.ConditionTrue,
	}, {
		name:                "broker exist sad",
		markBrokerExists:    &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "broker ready sad",
		markBrokerExists:    &trueValue,
		brokerStatus:        TestHelper.FalseBrokerStatus(),
		wantConditionStatus: corev1.ConditionFalse,
	}, {
		name:                "broker ready unknown",
		markBrokerExists:    &trueValue,
		brokerStatus:        TestHelper.UnknownBrokerStatus(),
		wantConditionStatus: corev1.ConditionUnknown,
	}, {
		name:                "all sad",
		markBrokerExists:    &falseValue,
		brokerStatus:        nil,
		wantConditionStatus: corev1.ConditionFalse,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ets := &v1beta1.EventTypeStatus{}
			if test.markBrokerExists != nil {
				if *test.markBrokerExists {
					ets.MarkBrokerExists()
				} else {
					ets.MarkBrokerDoesNotExist()
				}
			}
			if test.brokerStatus != nil {
				PropagateV1Alpha1BrokerStatus(ets, test.brokerStatus)
			}

			got := ets.GetTopLevelCondition().Status
			if test.wantConditionStatus != got {
				t.Errorf("unexpected readiness: want %v, got %v", test.wantConditionStatus, got)
			}
		})
	}
}

// Since v1beta1 to v1alpha1 is lossy but semantically equivalent,
// fix that so diff works.
func fixBrokerDeprecated(in *Broker) *Broker {
	in.Spec.ChannelTemplate = nil
	in.Status.TriggerChannel = nil
	return in
}
