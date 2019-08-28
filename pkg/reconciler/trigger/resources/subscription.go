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
	"net/url"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/utils"
)

// NewSubscription returns a placeholder subscription for trigger 't', from brokerTrigger to 'uri'
// replying to brokerIngress.
func NewSubscription(t *eventingv1alpha1.Trigger, brokerTrigger, brokerIngress *corev1.ObjectReference, uri *url.URL) *messagingv1alpha1.Subscription {
	uriString := uri.String()
	return &messagingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.Namespace,
			Name:      utils.GenerateFixedName(t, fmt.Sprintf("%s-%s", t.Spec.Broker, t.Name)),
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
			Subscriber: &messagingv1alpha1.SubscriberSpec{
				URI: &uriString,
			},
			Reply: &messagingv1alpha1.ReplyStrategy{
				Channel: &corev1.ObjectReference{
					APIVersion: brokerIngress.APIVersion,
					Kind:       brokerIngress.Kind,
					Name:       brokerIngress.Name,
				},
			},
		},
	}
}

// SubscriptionLabels generates the labels present on the Subscription linking this Trigger to the
// Broker's Channels.
func SubscriptionLabels(t *eventingv1alpha1.Trigger) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":  t.Spec.Broker,
		"eventing.knative.dev/trigger": t.Name,
	}
}
