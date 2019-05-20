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

package base

// crd contains functions that construct boilerplate CRD definitions.

import (
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const apiVersion = "eventing.knative.dev/v1alpha1"

// ClusterChannelProvisioner returns a ClusterChannelProvisioner for a given name.
func ClusterChannelProvisioner(name string) *corev1.ObjectReference {
	if name == "" {
		return nil
	}
	return pkgTest.CoreV1ObjectReference("ClusterChannelProvisioner", apiVersion, name)
}

// ChannelRef returns an ObjectReference for a given Channel Name.
func ChannelRef(name string) *corev1.ObjectReference {
	return pkgTest.CoreV1ObjectReference("Channel", apiVersion, name)
}

// Channel returns a Channel with the specified provisioner.
func Channel(name string, namespace string, provisioner *corev1.ObjectReference) *v1alpha1.Channel {
	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ChannelSpec{
			Provisioner: provisioner,
		},
	}
}

// SubscriberSpecForService returns a SubscriberSpec for a given Kubernetes Service.
func SubscriberSpecForService(name string) *v1alpha1.SubscriberSpec {
	if name == "" {
		return nil
	}
	return &v1alpha1.SubscriberSpec{
		Ref: pkgTest.CoreV1ObjectReference("Service", "v1", name),
	}
}

// ReplyStrategyForChannel returns a ReplyStrategy for a given Channel.
func ReplyStrategyForChannel(name string) *v1alpha1.ReplyStrategy {
	if name == "" {
		return nil
	}
	return &v1alpha1.ReplyStrategy{
		Channel: pkgTest.CoreV1ObjectReference("Channel", apiVersion, name),
	}
}

// Subscription returns a Subscription.
func Subscription(name string, namespace string, channel *corev1.ObjectReference, subscriber *v1alpha1.SubscriberSpec, reply *v1alpha1.ReplyStrategy) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel:    *channel,
			Subscriber: subscriber,
			Reply:      reply,
		},
	}
}

// Broker returns a Broker.
func Broker(name, namespace string, provisioner *corev1.ObjectReference) *v1alpha1.Broker {
	return &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.BrokerSpec{
			ChannelTemplate: &v1alpha1.ChannelSpec{
				Provisioner: provisioner,
			},
		},
	}
}

// TriggerFilter returns a TriggerFilter.
func TriggerFilter(eventSource, eventType string) *v1alpha1.TriggerFilter {
	return &v1alpha1.TriggerFilter{
		SourceAndType: &v1alpha1.TriggerFilterSourceAndType{
			Type:   eventType,
			Source: eventSource,
		},
	}
}

// Trigger returns a Trigger.
func Trigger(name, namespace, brokerName string, triggerFilter *v1alpha1.TriggerFilter, subscriber *v1alpha1.SubscriberSpec) *v1alpha1.Trigger {
	return &v1alpha1.Trigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: eventingv1alpha1.TriggerSpec{
			Broker:     brokerName,
			Filter:     triggerFilter,
			Subscriber: subscriber,
		},
	}
}
