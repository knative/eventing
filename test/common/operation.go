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

package common

import (
	"fmt"
	"time"

	"github.com/knative/eventing/test/base"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	pkgTest "github.com/knative/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelNamespace labels the given namespace with the labels map.
func (client *Client) LabelNamespace(labels map[string]string) error {
	namespace := client.Namespace
	nsSpec, err := client.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if nsSpec.Labels == nil {
		nsSpec.Labels = map[string]string{}
	}
	for k, v := range labels {
		nsSpec.Labels[k] = v
	}
	_, err = client.Kube.Kube.CoreV1().Namespaces().Update(nsSpec)
	return err
}

// SendFakeEventToChannel will send the given event to the given channel.
func (client *Client) SendFakeEventToChannel(senderName, channelName string, event *base.CloudEvent) error {
	namespace := client.Namespace
	channel, err := client.Eventing.EventingV1alpha1().Channels(namespace).Get(channelName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return client.sendFakeEventToAddressable(senderName, channel.Status.Address, event)
}

// SendFakeEventToBroker will send the given event to the given broker.
func (client *Client) SendFakeEventToBroker(senderName, brokerName string, event *base.CloudEvent) error {
	namespace := client.Namespace
	broker, err := client.Eventing.EventingV1alpha1().Brokers(namespace).Get(brokerName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	return client.sendFakeEventToAddressable(senderName, broker.Status.Address, event)
}

// sendFakeEventToAddressable will create a sender pod, which will send the given event to the given url.
func (client *Client) sendFakeEventToAddressable(
	senderName string,
	addr duckv1alpha1.Addressable,
	event *base.CloudEvent,
) error {
	namespace := client.Namespace
	url := fmt.Sprintf("http://%s", addr.Hostname)
	client.Logf("Sending fake CloudEvent")
	pod := base.EventSenderPod(senderName, url, event)
	if err := client.CreatePod(pod); err != nil {
		return err
	}
	if err := pkgTest.WaitForPodRunning(client.Kube, senderName, namespace); err != nil {
		return err
	}
	return nil
}

// WaitForBrokerReady waits until the broker is Ready.
func (client *Client) WaitForBrokerReady(name string) error {
	namespace := client.Namespace
	brokerMeta := base.Meta(name, namespace, "Broker")
	if err := base.WaitForResourceReady(client.Dynamic, brokerMeta); err != nil {
		return err
	}
	return nil
}

// WaitForBrokersReady waits until all brokers in the namespace are Ready.
func (client *Client) WaitForBrokersReady() error {
	namespace := client.Namespace
	brokers, err := client.Eventing.EventingV1alpha1().Brokers(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, broker := range brokers.Items {
		if err := client.WaitForBrokerReady(broker.Name); err != nil {
			return err
		}
	}
	return nil
}

// WaitForTriggerReady waits until the trigger is Ready.
func (client *Client) WaitForTriggerReady(name string) error {
	namespace := client.Namespace
	triggerMeta := base.Meta(name, namespace, "Trigger")
	if err := base.WaitForResourceReady(client.Dynamic, triggerMeta); err != nil {
		return err
	}
	return nil
}

// WaitForTriggersReady waits until all triggers in the namespace are Ready.
func (client *Client) WaitForTriggersReady() error {
	namespace := client.Namespace
	triggers, err := client.Eventing.EventingV1alpha1().Triggers(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, trigger := range triggers.Items {
		if err := client.WaitForTriggerReady(trigger.Name); err != nil {
			return err
		}
	}
	return nil
}

// WaitForChannelReady waits until the channel is Ready.
func (client *Client) WaitForChannelReady(name string) error {
	namespace := client.Namespace
	channelMeta := base.Meta(name, namespace, "Channel")
	if err := base.WaitForResourceReady(client.Dynamic, channelMeta); err != nil {
		return err
	}
	return nil
}

// WaitForChannelsReady waits until all channels in the namespace are Ready.
func (client *Client) WaitForChannelsReady() error {
	namespace := client.Namespace
	channels, err := client.Eventing.EventingV1alpha1().Channels(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, channel := range channels.Items {
		if err := client.WaitForChannelReady(channel.Name); err != nil {
			return err
		}
	}
	return nil
}

// WaitForSubscriptionReady waits until the subscription is Ready.
func (client *Client) WaitForSubscriptionReady(name string) error {
	namespace := client.Namespace
	subscriptionMeta := base.Meta(name, namespace, "Subscription")
	if err := base.WaitForResourceReady(client.Dynamic, subscriptionMeta); err != nil {
		return err
	}
	return nil
}

// WaitForSubscriptionsReady waits until all subscriptions in the namespace are Ready.
func (client *Client) WaitForSubscriptionsReady() error {
	namespace := client.Namespace
	subscriptions, err := client.Eventing.EventingV1alpha1().Subscriptions(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, subscription := range subscriptions.Items {
		if err := client.WaitForSubscriptionReady(subscription.Name); err != nil {
			return err
		}
	}
	return nil
}

// WaitForAllTestResourcesReady waits until all test resources in the namespace are Ready.
// Currently the test resources include Pod, Channel, Subscription, Broker and Trigger.
// If there are new resources, this function needs to be changed.
func (client *Client) WaitForAllTestResourcesReady() error {
	if err := client.WaitForChannelsReady(); err != nil {
		return err
	}
	if err := client.WaitForSubscriptionsReady(); err != nil {
		return err
	}
	if err := client.WaitForBrokersReady(); err != nil {
		return err
	}
	if err := client.WaitForTriggersReady(); err != nil {
		return err
	}
	if err := pkgTest.WaitForAllPodsRunning(client.Kube, client.Namespace); err != nil {
		return err
	}
	// FIXME(Fredy-Z): This hacky sleep is added to try mitigating the test flakiness.
	// Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)
	return nil
}
