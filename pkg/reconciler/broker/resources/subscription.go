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
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// NewSubscription returns a placeholder subscription for trigger 't', from brokerTrigger to 'dest'
// replying to brokerIngress.
func NewSubscription(ctx context.Context, t *eventingv1.Trigger, brokerTrigger *corev1.ObjectReference, dest, reply *duckv1.Destination, delivery *eventingduckv1.DeliverySpec) *messagingv1.Subscription {
	var broker, channelNamespace string
	if t.Spec.BrokerRef != nil && feature.FromContext(ctx).IsEnabled(feature.CrossNamespaceEventLinks) {
		broker = t.Spec.BrokerRef.Name
		channelNamespace = t.Spec.BrokerRef.Namespace
	} else {
		broker = t.Spec.Broker
		channelNamespace = ""
	}
	return &messagingv1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: t.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("%s-%s-", broker, t.Name), string(t.GetUID())),
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(t),
			},
			Labels: SubscriptionLabels(ctx, t),
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: duckv1.KReference{
				APIVersion: brokerTrigger.APIVersion,
				Kind:       brokerTrigger.Kind,
				Name:       brokerTrigger.Name,
				Namespace:  channelNamespace,
			},
			Subscriber: dest,
			Reply:      reply,
			Delivery:   delivery,
		},
	}
}

// SubscriptionLabels generates the labels present on the Subscription linking this Trigger to the
// Broker's Channels.
func SubscriptionLabels(ctx context.Context, t *eventingv1.Trigger) map[string]string {
	var broker string
	if t.Spec.BrokerRef != nil && feature.FromContext(ctx).IsEnabled(feature.CrossNamespaceEventLinks) {
		broker = t.Spec.BrokerRef.Name
	} else {
		broker = t.Spec.Broker
	}
	return map[string]string{
		eventing.BrokerLabelKey:        broker,
		"eventing.knative.dev/trigger": t.Name,
	}
}
