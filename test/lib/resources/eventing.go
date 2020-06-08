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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgTest "knative.dev/pkg/test"

	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
)

// BrokerV1Beta1Option enables further configuration of a Broker.
type BrokerV1Beta1Option func(*eventingv1beta1.Broker)

// TriggerOptionV1Beta1 enables further configuration of a v1beta1 Trigger.
type TriggerOptionV1Beta1 func(*eventingv1beta1.Trigger)

// SubscriptionOptionV1Beta1 enables further configuration of a Subscription.
type SubscriptionOptionV1Beta1 func(*messagingv1beta1.Subscription)

// DeliveryOption enables further configuration of DeliverySpec.
type DeliveryOption func(*eventingduckv1beta1.DeliverySpec)

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

func KnativeRefForBroker(name, namespace string) *duckv1.KReference {
	return &duckv1.KReference{
		Kind:       "Broker",
		APIVersion: "eventing.knative.dev/v1beta1",
		Name:       name,
		Namespace:  namespace,
	}
}

// WithSubscriberForSubscription returns an option that adds a Subscriber for the given
// v1beta1 Subscription.
func WithSubscriberForSubscription(name string) SubscriptionOptionV1Beta1 {
	return func(s *messagingv1beta1.Subscription) {
		if name != "" {
			s.Spec.Subscriber = &duckv1.Destination{
				Ref: KnativeRefForService(name, ""),
			}
		}
	}
}

// WithReplyForSubscription returns an options that adds a ReplyStrategy for the given Subscription.
func WithReplyForSubscription(name string, typemeta *metav1.TypeMeta) SubscriptionOptionV1Beta1 {
	return func(s *messagingv1beta1.Subscription) {
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
func WithDeadLetterSinkForSubscription(name string) SubscriptionOptionV1Beta1 {
	return func(s *messagingv1beta1.Subscription) {
		if name != "" {
			delivery := s.Spec.Delivery
			if delivery == nil {
				delivery = &eventingduckv1beta1.DeliverySpec{}
				s.Spec.Delivery = delivery
			}

			delivery.DeadLetterSink = &duckv1.Destination{
				Ref: KnativeRefForService(name, s.Namespace),
			}

		}
	}
}

// SubscriptionV1Beta1 returns a v1beta1 Subscription.
func SubscriptionV1Beta1(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...SubscriptionOptionV1Beta1,
) *messagingv1beta1.Subscription {
	subscription := &messagingv1beta1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: messagingv1beta1.SubscriptionSpec{
			Channel: *channelRef(channelName, channelTypeMeta),
		},
	}
	for _, option := range options {
		option(subscription)
	}
	return subscription
}

// WithConfigMapForBrokerConfig returns a function that configures the ConfigMap
// for the Spec.Config for a given Broker. Note that the CM must exist and has
// to be in the same namespace as the Broker and have the same Name. Typically
// you'd do this by calling client.CreateBrokerConfigMapOrFail and then call this
// method.
// If those don't apply to your ConfigMap, look at WithConfigForBrokerV1Beta1
func WithConfigMapForBrokerConfig() BrokerV1Beta1Option {
	return func(b *eventingv1beta1.Broker) {
		b.Spec.Config = &duckv1.KReference{
			Name:       b.Name,
			Namespace:  b.Namespace,
			Kind:       "ConfigMap",
			APIVersion: "v1",
		}
	}
}

func WithConfigForBrokerV1Beta1(config *duckv1.KReference) BrokerV1Beta1Option {
	return func(b *eventingv1beta1.Broker) {
		b.Spec.Config = config
	}
}

// WithBrokerClassForBrokerV1Beta1 returns a function that adds a brokerClass
// annotation to the given Broker.
func WithBrokerClassForBrokerV1Beta1(brokerClass string) BrokerV1Beta1Option {
	return func(b *eventingv1beta1.Broker) {
		annotations := b.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string, 1)
		}
		annotations["eventing.knative.dev/broker.class"] = brokerClass
		b.SetAnnotations(annotations)
	}
}

// WithDeliveryForBrokerV1Beta1 returns a function that adds a Delivery for the given
// v1beta1 Broker.
func WithDeliveryForBrokerV1Beta1(delivery *eventingduckv1beta1.DeliverySpec) BrokerV1Beta1Option {
	return func(b *eventingv1beta1.Broker) {
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

// Broker returns a Broker.
func BrokerV1Beta1(name string, options ...BrokerV1Beta1Option) *eventingv1beta1.Broker {
	broker := &eventingv1beta1.Broker{
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
func WithAttributesTriggerFilterV1Beta1(eventSource, eventType string, extensions map[string]interface{}) TriggerOptionV1Beta1 {
	attrs := make(map[string]string)
	if eventType != "" {
		attrs["type"] = eventType
	} else {
		attrs["type"] = eventingv1beta1.TriggerAnyFilter
	}
	if eventSource != "" {
		attrs["source"] = eventSource
	} else {
		attrs["source"] = eventingv1beta1.TriggerAnyFilter
	}
	for k, v := range extensions {
		attrs[k] = fmt.Sprintf("%v", v)
	}
	return func(t *eventingv1beta1.Trigger) {
		t.Spec.Filter = &eventingv1beta1.TriggerFilter{
			Attributes: eventingv1beta1.TriggerFilterAttributes(attrs),
		}
	}
}

// WithDependencyAnnotationTrigger returns an option that adds a dependency annotation to the given Trigger.
func WithDependencyAnnotationTriggerV1Beta1(dependencyAnnotation string) TriggerOptionV1Beta1 {
	return func(t *eventingv1beta1.Trigger) {
		if t.Annotations == nil {
			t.Annotations = make(map[string]string)
		}
		t.Annotations[eventingv1beta1.DependencyAnnotation] = dependencyAnnotation
	}
}

// WithSubscriberServiceRefForTriggerV1Beta1 returns an option that adds a Subscriber Knative Service Ref for the given Trigger.
func WithSubscriberServiceRefForTriggerV1Beta1(name string) TriggerOptionV1Beta1 {
	return func(t *eventingv1beta1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = duckv1.Destination{
				Ref: KnativeRefForService(name, t.Namespace),
			}
		}
	}
}

// WithSubscriberURIForTriggerV1Beta1 returns an option that adds a Subscriber URI for the given Trigger.
func WithSubscriberURIForTriggerV1Beta1(uri string) TriggerOptionV1Beta1 {
	apisURI, _ := apis.ParseURL(uri)
	return func(t *eventingv1beta1.Trigger) {
		t.Spec.Subscriber = duckv1.Destination{
			URI: apisURI,
		}
	}
}

// WithBrokerV1Beta1 returns an option that adds a broker for the given Trigger.
func WithBrokerV1Beta1(name string) TriggerOptionV1Beta1 {
	return func(trigger *eventingv1beta1.Trigger) {
		trigger.Spec.Broker = name
	}
}

// TriggerV1Beta1 returns a v1beta1 Trigger.
func TriggerV1Beta1(name string, options ...TriggerOptionV1Beta1) *eventingv1beta1.Trigger {
	trigger := &eventingv1beta1.Trigger{
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
