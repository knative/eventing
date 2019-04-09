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

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NewSubscription returns a placeholder subscription for trigger 't', from brokerTrigger to 'svc'
// replying to brokerIngress.
func NewSubscription(t *eventingv1alpha1.Trigger, brokerTrigger, brokerIngress *eventingv1alpha1.Channel, svc *corev1.Service) *eventingv1alpha1.Subscription {
	return &eventingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    t.Namespace,
			GenerateName: fmt.Sprintf("%s-%s-", t.Spec.Broker, t.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Trigger",
				}),
			},
			Labels: SubscriptionLabels(t),
		},
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
				Name:       brokerTrigger.Name,
			},
			Subscriber: &eventingv1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       svc.Name,
				},
			},
			Reply: &eventingv1alpha1.ReplyStrategy{
				Channel: &corev1.ObjectReference{
					APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
					Kind:       "Channel",
					Name:       brokerIngress.Name,
				},
			},
		},
	}
}
