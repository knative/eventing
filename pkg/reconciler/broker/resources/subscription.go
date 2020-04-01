/*
Copyright 2019 The Knative Authors

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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmeta"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// MakeSubscription returns a placeholder subscription for broker 'b', channelable 'c', and service 'svc'.
func MakeSubscription(b *v1alpha1.Broker, c *duckv1alpha1.Channelable, svc *corev1.Service) *messagingv1alpha1.Subscription {
	return &messagingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: b.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("internal-ingress-%s", b.Name), string(b.GetUID())),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(b),
			},
			Labels: ingressSubscriptionLabels(b.Name),
		},
		Spec: messagingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: c.APIVersion,
				Kind:       c.Kind,
				Name:       c.Name,
			},
			Subscriber: &duckv1.Destination{
				Ref: &duckv1.KReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       svc.Name,
					Namespace:  svc.Namespace,
				},
			},
		},
	}
}

func ingressSubscriptionLabels(brokerName string) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:              brokerName,
		"eventing.knative.dev/brokerIngress": "true",
	}
}

// NewSubscription returns a placeholder subscription for trigger 't', from brokerTrigger to 'uri'
// replying to brokerIngress.
func NewSubscription(t *v1alpha1.Trigger, brokerTrigger, brokerRef *corev1.ObjectReference, uri *apis.URL, delivery *duckv1beta1.DeliverySpec) *messagingv1alpha1.Subscription {
	return &messagingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("%s-%s-", t.Spec.Broker, t.Name), string(t.GetUID())),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(t),
			},
			Labels: SubscriptionLabels(t),
		},
		Spec: messagingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: brokerTrigger.APIVersion,
				Kind:       brokerTrigger.Kind,
				Name:       brokerTrigger.Name,
			},
			Subscriber: &duckv1.Destination{
				URI: uri,
			},
			Reply: &duckv1.Destination{
				Ref: &duckv1.KReference{
					APIVersion: brokerRef.APIVersion,
					Kind:       brokerRef.Kind,
					Name:       brokerRef.Name,
					Namespace:  brokerRef.Namespace,
				},
			},
			Delivery: delivery,
		},
	}
}

// SubscriptionLabels generates the labels present on the Subscription linking this Trigger to the
// Broker's Channels.
func SubscriptionLabels(t *v1alpha1.Trigger) map[string]string {
	return map[string]string{
		eventing.BrokerLabelKey:        t.Spec.Broker,
		"eventing.knative.dev/trigger": t.Name,
	}
}
