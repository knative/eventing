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

// This file contains functions that construct Eventing resources.

import (
	"fmt"

	"knative.dev/pkg/apis"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"

	configsv1alpha1 "knative.dev/eventing/pkg/apis/configs/v1alpha1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/reconciler/namespace/resources"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*eventingv1alpha1.Broker)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*eventingv1alpha1.Trigger)

// SubscriptionOption enables further configuration of a Subscription.
type SubscriptionOption func(*messagingv1alpha1.Subscription)

// DeliveryOption enables further configuration of DeliverySpec.
type DeliveryOption func(*eventingduckv1alpha1.DeliverySpec)

// channelRef returns an ObjectReference for a given Channel name.
func channelRef(name string, typemeta *metav1.TypeMeta) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference(typemeta.Kind, typemeta.APIVersion, name)
}

func KnativeRefForService(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Service",
		APIVersion: "v1",
		Name:       name,
		Namespace:  namespace,
	}
}

// WithSubscriberForSubscription returns an option that adds a Subscriber for the given Subscription.
func WithSubscriberForSubscription(name string) SubscriptionOption {
	return func(s *messagingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Subscriber = &duckv1.Destination{
				Ref: KnativeRefForService(name, ""),
			}
		}
	}
}

// WithReplyForSubscription returns an options that adds a ReplyStrategy for the given Subscription.
func WithReplyForSubscription(name string, typemeta *metav1.TypeMeta) SubscriptionOption {
	return func(s *messagingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Reply = &duckv1.Destination{
				Ref: &duckv1.KReference{
					Kind:       typemeta.Kind,
					APIVersion: typemeta.APIVersion,
					Name:       name,
					Namespace:  s.Namespace},
			}
		}
	}
}

// WithDeadLetterSinkForSubscription returns an options that adds a DeadLetterSink for the given Subscription.
func WithDeadLetterSinkForSubscription(name string) SubscriptionOption {
	return func(s *messagingv1alpha1.Subscription) {
		if name != "" {
			delivery := s.Spec.Delivery
			if delivery == nil {
				delivery = &eventingduckv1alpha1.DeliverySpec{}
				s.Spec.Delivery = delivery
			}

			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: KnativeRefForService(name, s.Namespace),
			}

		}
	}
}

// Subscription returns a Subscription.
func Subscription(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...SubscriptionOption,
) *messagingv1alpha1.Subscription {
	subscription := &messagingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: messagingv1alpha1.SubscriptionSpec{
			Channel: *channelRef(channelName, channelTypeMeta),
		},
	}
	for _, option := range options {
		option(subscription)
	}
	return subscription
}

// WithChannelTemplateForBroker returns a function that adds a ChannelTemplate for the given Broker.
func WithChannelTemplateForBroker(channelTypeMeta *metav1.TypeMeta) BrokerOption {
	return func(b *eventingv1alpha1.Broker) {
		channelTemplate := &eventingduckv1alpha1.ChannelTemplateSpec{
			TypeMeta: *channelTypeMeta,
		}
		b.Spec.ChannelTemplate = channelTemplate
	}
}

// WithDeliveryForBroker returns a function that adds a Delivery for the given Broker.
func WithDeliveryForBroker(delivery *eventingduckv1alpha1.DeliverySpec) BrokerOption {
	return func(b *eventingv1alpha1.Broker) {
		b.Spec.Delivery = delivery
	}
}

// ConfigMapPropagation returns a ConfigMapPropagation.
func ConfigMapPropagation(name, namespace string) *configsv1alpha1.ConfigMapPropagation {
	return &configsv1alpha1.ConfigMapPropagation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: configsv1alpha1.ConfigMapPropagationSpec{
			OriginalNamespace: "knative-eventing",
			Selector: &metav1.LabelSelector{
				MatchLabels: resources.ConfigMapPropagationOwnedLabels(),
			},
		},
	}
}

// ConfigMap returns a ConfigMap.
func ConfigMap(name string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				// Filter label for a configmap being picked to be propagated by current configmappropagation.
				resources.CmpDefaultLabelKey: resources.CmpDefaultLabelValue,
				// Default label for a configmap being eligible to be propagated.
				"knative.dev/config-propagation": "original",
			},
		},
		Data: data,
	}
}

// Broker returns a Broker.
func Broker(name string, options ...BrokerOption) *eventingv1alpha1.Broker {
	broker := &eventingv1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(broker)
	}
	return broker
}

// WithDeprecatedSourceAndTypeTriggerFilter returns an option that adds a TriggerFilter with DeprecatedSourceAndType for the given Trigger.
func WithDeprecatedSourceAndTypeTriggerFilter(eventSource, eventType string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		triggerFilter := &eventingv1alpha1.TriggerFilter{
			DeprecatedSourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
				Type:   eventType,
				Source: eventSource,
			},
		}
		t.Spec.Filter = triggerFilter
	}
}

// WithAttributesTriggerFilter returns an option that adds a TriggerFilter with Attributes for the given Trigger.
func WithAttributesTriggerFilter(eventSource, eventType string, extensions map[string]interface{}) TriggerOption {
	attrs := make(map[string]string)
	attrs["type"] = eventType
	attrs["source"] = eventSource
	for k, v := range extensions {
		attrs[k] = fmt.Sprintf("%v", v)
	}
	triggerFilterAttributes := eventingv1alpha1.TriggerFilterAttributes(attrs)
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Filter = &eventingv1alpha1.TriggerFilter{
			Attributes: &triggerFilterAttributes,
		}
	}
}

// WithBroker returns an option that adds a Broker for the given Trigger.
func WithBroker(brokerName string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Broker = brokerName
	}
}

// WithSubscriberKServiceRefForTrigger returns an option that adds a Subscriber Knative Service Ref for the given Trigger.
func WithSubscriberKServiceRefForTrigger(name string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = duckv1.Destination{
				Ref: KnativeRefForService(name, t.Namespace),
			}
		}
	}
}

// WithSubscriberServiceRefForTrigger returns an option that adds a Subscriber Kubernetes Service Ref for the given Trigger.
func WithSubscriberServiceRefForTrigger(name string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = duckv1.Destination{
				Ref: KnativeRefForService(name, t.Namespace),
			}
		}
	}
}

// WithSubscriberURIForTrigger returns an option that adds a Subscriber URI for the given Trigger.
func WithSubscriberURIForTrigger(uri string) TriggerOption {
	apisURI, _ := apis.ParseURL(uri)
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			URI: apisURI,
		}
	}
}

// Trigger returns a Trigger.
func Trigger(name string, options ...TriggerOption) *eventingv1alpha1.Trigger {
	trigger := &eventingv1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(trigger)
	}
	return trigger
}

// WithDeadLetterSinkForDelivery returns an options that adds a DeadLetterSink for the given DeliverySpec.
func WithDeadLetterSinkForDelivery(name string) DeliveryOption {
	return func(delivery *eventingduckv1alpha1.DeliverySpec) {
		if name != "" {
			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: KnativeRefForService(name, ""),
			}
		}
	}
}

// Delivery returns a DeliverySpec.
func Delivery(options ...DeliveryOption) *eventingduckv1alpha1.DeliverySpec {
	delivery := &eventingduckv1alpha1.DeliverySpec{}
	for _, option := range options {
		option(delivery)
	}
	return delivery
}
