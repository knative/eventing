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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	pkgTest "knative.dev/pkg/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BrokerOption enables further configuration of a Broker.
type BrokerOption func(*eventingv1alpha1.Broker)

// TriggerOption enables further configuration of a Trigger.
type TriggerOption func(*eventingv1alpha1.Trigger)

// SubscriptionOption enables further configuration of a Subscription.
type SubscriptionOption func(*eventingv1alpha1.Subscription)

// clusterChannelProvisioner returns a ClusterChannelProvisioner for a given name.
func clusterChannelProvisioner(name string) *corev1.ObjectReference {
	if name == "" {
		return nil
	}
	return pkgTest.CoreV1ObjectReference(ClusterChannelProvisionerKind, EventingAPIVersion, name)
}

// channelRef returns an ObjectReference for a given Channel Name.
func channelRef(name string, typemeta *metav1.TypeMeta) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference(typemeta.Kind, typemeta.APIVersion, name)
}

// Channel returns a Channel with the specified provisioner.
func Channel(name, provisioner string) *eventingv1alpha1.Channel {
	return &eventingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eventingv1alpha1.ChannelSpec{
			Provisioner: clusterChannelProvisioner(provisioner),
		},
	}
}

// WithSubscriberForSubscription returns an option that adds a Subscriber for the given Subscription.
func WithSubscriberForSubscription(name string) SubscriptionOption {
	return func(s *eventingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
				Ref: pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name),
			}
		}
	}
}

// WithReply returns an options that adds a ReplyStrategy for the given Subscription.
func WithReply(name string, typemeta *metav1.TypeMeta) SubscriptionOption {
	return func(s *eventingv1alpha1.Subscription) {
		if name != "" {
			s.Spec.Reply = &eventingv1alpha1.ReplyStrategy{
				Channel: pkgTest.CoreV1ObjectReference(typemeta.Kind, typemeta.APIVersion, name),
			}
		}
	}
}

// Subscription returns a Subscription.
func Subscription(
	name, channelName string,
	channelTypeMeta *metav1.TypeMeta,
	options ...SubscriptionOption,
) *eventingv1alpha1.Subscription {
	subscription := &eventingv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: eventingv1alpha1.SubscriptionSpec{
			Channel: *channelRef(channelName, channelTypeMeta),
		},
	}
	for _, option := range options {
		option(subscription)
	}
	return subscription
}

// WithDeprecatedChannelTemplateForBroker returns a function that adds a DeprecatedChannelTemplate for the given Broker.
func WithDeprecatedChannelTemplateForBroker(provisionerName string) BrokerOption {
	return func(b *eventingv1alpha1.Broker) {
		deprecatedChannelTemplate := &eventingv1alpha1.ChannelSpec{
			Provisioner: clusterChannelProvisioner(provisionerName),
		}
		b.Spec.DeprecatedChannelTemplate = deprecatedChannelTemplate
	}
}

// WithChannelTemplateForBroker returns a function that adds a ChannelTemplate for the given Broker.
func WithChannelTemplateForBroker(channelTypeMeta metav1.TypeMeta) BrokerOption {
	return func(b *eventingv1alpha1.Broker) {
		channelTemplate := eventingv1alpha1.ChannelTemplateSpec{
			TypeMeta: channelTypeMeta,
		}
		b.Spec.ChannelTemplate = channelTemplate
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

// WithTriggerFilter returns an option that adds a TriggerFilter for the given Trigger.
func WithTriggerFilter(eventSource, eventType string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		triggerFilter := &eventingv1alpha1.TriggerFilter{
			SourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
				Type:   eventType,
				Source: eventSource,
			},
		}
		t.Spec.Filter = triggerFilter
	}
}

// WithBroker returns an option that adds a Broker for the given Trigger.
func WithBroker(brokerName string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Broker = brokerName
	}
}

// WithSubscriberRefForTrigger returns an option that adds a Subscriber Ref for the given Trigger.
func WithSubscriberRefForTrigger(name string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		if name != "" {
			t.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
				Ref: pkgTest.CoreV1ObjectReference(ServiceKind, CoreAPIVersion, name),
			}
		}
	}
}

// WithSubscriberURIForTrigger returns an option that adds a Subscriber URI for the given Trigger.
func WithSubscriberURIForTrigger(uri string) TriggerOption {
	return func(t *eventingv1alpha1.Trigger) {
		t.Spec.Subscriber = &eventingv1alpha1.SubscriberSpec{
			URI: &uri,
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
