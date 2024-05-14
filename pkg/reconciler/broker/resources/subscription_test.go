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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
)

func TestNewSubscription(t *testing.T) {
	var TrueValue = true
	trigger := &eventingv1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "t-namespace",
			Name:      "t-name",
		},
		Spec: eventingv1.TriggerSpec{
			Broker: "broker-name",
		},
	}
	triggerChannelRef := &corev1.ObjectReference{
		Name:       "tc-name",
		Kind:       "tc-kind",
		APIVersion: "tc-apiVersion",
	}
	delivery := &eventingduckv1.DeliverySpec{
		DeadLetterSink: &duckv1.Destination{
			URI: apis.HTTP("dlc.example.com"),
		},
	}
	dest := &duckv1.Destination{
		URI: apis.HTTP("example.com"),
	}
	reply := &duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       "broker-name",
			Namespace:  "t-namespace",
			Kind:       "broker-kind",
			APIVersion: "broker-apiVersion",
		},
	}
	got := NewSubscription(trigger, triggerChannelRef, dest, reply, delivery)
	want := &messagingv1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "t-namespace",
			Name:      "broker-name-t-name-",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         "eventing.knative.dev/v1",
				Kind:               "Trigger",
				Name:               "t-name",
				Controller:         &TrueValue,
				BlockOwnerDeletion: &TrueValue,
			}},
			Labels: map[string]string{
				eventing.BrokerLabelKey:        "broker-name",
				"eventing.knative.dev/trigger": "t-name",
			},
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: duckv1.KReference{
				Name:       "tc-name",
				Kind:       "tc-kind",
				APIVersion: "tc-apiVersion",
			},
			Subscriber: &duckv1.Destination{
				URI: apis.HTTP("example.com"),
			},
			Reply: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Name:       "broker-name",
					Namespace:  "t-namespace",
					Kind:       "broker-kind",
					APIVersion: "broker-apiVersion",
				},
			},
			Delivery: &eventingduckv1.DeliverySpec{
				DeadLetterSink: &duckv1.Destination{
					URI: apis.HTTP("dlc.example.com"),
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Error("unexpected diff (-want, +got) =", diff)
	}
}
