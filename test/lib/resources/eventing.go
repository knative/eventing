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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// BrokerOption enables further configuration of a v1 Broker.
type BrokerOption func(*eventingv1.Broker)

// TriggerOption enables further configuration of a v1 Trigger.
type TriggerOption func(*eventingv1.Trigger)

// SubscriptionOption enables further configuration of a v1 Subscription.
type SubscriptionOption func(*messagingv1.Subscription)

// DeliveryOption enables further configuration of DeliverySpec.
type DeliveryOption func(*eventingduckv1beta1.DeliverySpec)

// channelRef returns an ObjectReference for a given Channel name.
func channelRef(name string, typemeta *metav1.TypeMeta) duckv1.KReference {
	return duckv1.KReference{
		Kind:       typemeta.Kind,
		APIVersion: typemeta.APIVersion,
		Name:       name,
	}
}

func KnativeRefForService(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Service",
		APIVersion: "v1",
		Name:       name,
		Namespace:  namespace,
	}
}

func KnativeRefForBroker(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1",
		Name:       name,
		Namespace:  namespace,
	}
}

// WithSubscriberForSubscription returns an option that adds a Subscriber for the given
// v1 Subscription.
func WithSubscriberForSubscription(name string) SubscriptionOption {
	return func(s *messagingv1.Subscription) {
		if name != "" {
			s.Spec.Subscriber = &duckv1.Destination{
				Ref: KnativeRefForService(name, ""),
			}
		}
	}
}

// WithURIForSubscription returns an option that adds a URI for the given v1 Subscription
func WithURIForSubscription(uri *apis.URL) SubscriptionOption {
	return func(s *messagingv1.Subscription) {
		if uri != nil {
			s.Spec.Subscriber = &duckv1.Destination{
				URI: uri,
			}
		}
	}
}

// WithReplyForSubscription returns an options that adds a ReplyStrategy for the given v1 Subscription.
func WithReplyForSubscription(name string, typemeta *metav1.TypeMeta) SubscriptionOption {
	return func(s *messagingv1.Subscription) {
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

// WithDeadLetterSinkForSubscription returns an options that adds a DeadLetterSink for the given v1 Subscription.
func WithDeadLetterSinkForSubscription(name string) SubscriptionOption {
	return func(s *messagingv1.Subscription) {
		if name != "" {
			delivery := s.Spec.Delivery
			if delivery == nil {
				delivery = &eventingduckv1.DeliverySpec{}
				s.Spec.Delivery = delivery
			}

			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: KnativeRefForService(name, s.Namespace),
			}

		}
	}
}

// SubscriptionV1 returns a v1 Subscription.
func Subscription(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...SubscriptionOption,
) *messagingv1.Subscription {
	subscription := &messagingv1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: messagingv1.SubscriptionSpec{
			Channel: channelRef(channelName, channelTypeMeta),
		},
	}
	for _, option := range options {
		option(subscription)
	}
	return subscription
}

func WithConfigForBroker(config *duckv1.KReference) BrokerOption {
	return func(b *eventingv1.Broker) {
		b.Spec.Config = config
	}
}

// WithConfigMapForBrokerConfig returns a function that configures the ConfigMap
// for the Spec.Config for a given Broker. Note that the CM must exist and has
// to be in the same namespace as the Broker and have the same Name. Typically
// you'd do this by calling client.CreateBrokerConfigMapOrFail and then call this
// method.
// If those don't apply to your ConfigMap, look at WithConfigForBrokerV1Beta1
func WithConfigMapForBrokerConfig() BrokerOption {
	return func(b *eventingv1.Broker) {
		b.Spec.Config = &duckv1.KReference{
			Name:       b.Name,
			Namespace:  b.Namespace,
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}
	}
}

// WithBrokerClassForBroker returns a function that adds a brokerClass
// annotation to the given v1 Broker.
func WithBrokerClassForBroker(brokerClass string) BrokerOption {
	return func(b *eventingv1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations["eventing.knative.dev/broker.class"] = brokerClass
		b.SetAnnotations(annotations)
	}
}

// WithCustomAnnotationForBroker returns a function that adds a custom
// annotation to the given v1 Broker.
func WithCustomAnnotationForBroker(annotationKey, annotationValue string) BrokerOption {
	return func(b *eventingv1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations[annotationKey] = annotationValue
		b.SetAnnotations(annotations)
	}
}

// WithDeliveryForBroker returns a function that adds a Delivery for the given
// v1 Broker.
func WithDeliveryForBroker(delivery *eventingduckv1.DeliverySpec) BrokerOption {
	return func(b *eventingv1.Broker) {
		b.Spec.Delivery = delivery
	}
}

// ConfigMap returns a ConfigMap.
func ConfigMap(name, namespace string, data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				// Default label for a configmap being eligible to be propagated.
				"knative.dev/config-propagation": "original",
			},
		},
		Data: data,
	}
}

// Broker returns a v1 Broker.
func Broker(name string, options ...BrokerOption) *eventingv1.Broker {
	broker := &eventingv1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	for _, option := range options {
		option(broker)
	}
	return broker
}

// WithAttributesTriggerFilter returns an option that adds a TriggerFilter with Attributes for the given Trigger.
func WithAttributesTriggerFilter(eventSource, eventType string, extensions map[string]interface{}) TriggerOption {
	attrs := make(map[string]string)
	if eventType != "" {
		attrs["type"] = eventType
	} else {
		attrs["type"] = eventingv1.TriggerAnyFilter
	}
	if eventSource != "" {
		attrs["source"] = eventSource
	} else {
		attrs["source"] = eventingv1.TriggerAnyFilter
	}
	for k, v := range extensions {
		attrs[k] = fmt.Sprintf("%v", v)
	}
	return func(t *eventingv1.Trigger) {
		t.Spec.Filter = &eventingv1.TriggerFilter{
			Attributes: eventingv1.TriggerFilterAttributes(attrs),
		}
	}
}

// WithDependencyAnnotationTrigger returns an option that adds a dependency annotation to the given Trigger.
func WithDependencyAnnotationTrigger(dependencyAnnotation string) TriggerOption {
	return func(t *eventingv1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[eventingv1.DependencyAnnotation] = dependencyAnnotation
	}
}

// WithSubscriberServiceRefForTrigger returns an option that adds a Subscriber Knative Service Ref for the given v1 Trigger.
func WithSubscriberServiceRefForTrigger(name string) TriggerOption {
	return WithSubscriberDestination(func(t *eventingv1.Trigger) duckv1.Destination {
		return duckv1.Destination{
			Ref: KnativeRefForService(name, t.Namespace),
		}
	})
}

// WithSubscriberURIForTrigger returns an option that adds a Subscriber URI for the given v1 Trigger.
func WithSubscriberURIForTrigger(uri string) TriggerOption {
	return WithSubscriberDestination(func(t *eventingv1.Trigger) duckv1.Destination {
		apisURI, _ := apis.ParseURL(uri)
		return duckv1.Destination{
			URI: apisURI,
		}
	})
}

// WithSubscriberDestination returns an option that adds a Subscriber for given
// duckv1.Destination.
func WithSubscriberDestination(destFactory func(t *eventingv1.Trigger) duckv1.Destination) TriggerOption {
	return func(t *eventingv1.Trigger) {
		dest := destFactory(t)
		if dest.Ref != nil || dest.URI != nil {
			t.Spec.Subscriber = dest
		}
	}
}

// WithBroker returns an option that adds a broker for the given Trigger.
func WithBroker(name string) TriggerOption {
	return func(trigger *eventingv1.Trigger) {
		trigger.Spec.Broker = name
	}
}

// Trigger returns a v1 Trigger.
func Trigger(name string, options ...TriggerOption) *eventingv1.Trigger {
	trigger := &eventingv1.Trigger{
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
	return func(delivery *eventingduckv1beta1.DeliverySpec) {
		if name != "" {
			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: KnativeRefForService(name, ""),
			}
		}
	}
}

// Delivery returns a DeliverySpec.
func Delivery(options ...DeliveryOption) *eventingduckv1beta1.DeliverySpec {
	delivery := &eventingduckv1beta1.DeliverySpec{}
	for _, option := range options {
		option(delivery)
	}
	return delivery
}
