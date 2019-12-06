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

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// NewSubscription returns a placeholder subscription for trigger 't', from brokerTrigger to 'uri'
// replying to brokerIngress.
func NewSubscription(t *eventingv1alpha1.Trigger, brokerTrigger, brokerRef *corev1.ObjectReference, uri *url.URL) *messagingv1alpha1.Subscription {
	// TODO: Figure out once Trigger moves to Destination how this changes.
	tmpURI, err := apis.ParseURL(uri.String())
	if err != nil {
		panic("should NEVER happen")
	}

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
			Subscriber: &duckv1.Destination{
				URI: tmpURI,
			},
			Reply: &duckv1.Destination{
				Ref: &corev1.ObjectReference{
					APIVersion: brokerRef.APIVersion,
					Kind:       brokerRef.Kind,
					Name:       brokerRef.Name,
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
