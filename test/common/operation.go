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
	pkgTest "github.com/knative/pkg/test"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelNamespace labels the given namespace with the labels map.
func (client *Client) LabelNamespace(labels map[string]string) error {
	namespace := client.Namespace
	nsSpec, err := client.Kube.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
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
	url := fmt.Sprintf("http://%s", channel.Status.Address.Hostname)
	return client.SendFakeEventToAddress(senderName, url, event)
}

// SendFakeEventToBroker will send the given event to the given broker.
func (client *Client) SendFakeEventToBroker(senderName, brokerName string, event *base.CloudEvent) error {
	namespace := client.Namespace
	broker, err := client.Eventing.EventingV1alpha1().Brokers(namespace).Get(brokerName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s", broker.Status.Address.Hostname)
	return client.SendFakeEventToAddress(senderName, url, event)
}

// SendFakeEventToAddress will create a sender pod, which will send the given event to the given url.
func (client *Client) SendFakeEventToAddress(senderName, url string, event *base.CloudEvent) error {
	namespace := client.Namespace
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
	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	if err := base.WaitForBrokerState(brokers, name, base.IsBrokerReady, "BrokerIsReady"); err != nil {
		return err
	}
	return nil
}

// WaitForBrokersReady waits until all brokers in the namespace are Ready.
func (client *Client) WaitForBrokersReady() error {
	namespace := client.Namespace
	brokers := client.Eventing.EventingV1alpha1().Brokers(namespace)
	if err := base.WaitForBrokerListState(brokers, base.BrokersReady, "BrokersAreReady"); err != nil {
		return err
	}
	return nil
}

// WaitForTriggerReady waits until the trigger is Ready.
func (client *Client) WaitForTriggerReady(name string) error {
	namespace := client.Namespace
	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	if err := base.WaitForTriggerState(triggers, name, base.IsTriggerReady, "TriggerIsReady"); err != nil {
		return err
	}
	return nil
}

// WaitForTriggersReady waits until all triggers in the namespace are Ready.
func (client *Client) WaitForTriggersReady() error {
	namespace := client.Namespace
	triggers := client.Eventing.EventingV1alpha1().Triggers(namespace)
	if err := base.WaitForTriggerListState(triggers, base.TriggersReady, "TriggersAreReady"); err != nil {
		return err
	}
	return nil
}

// WaitForChannelReady waits until the channel is Ready.
func (client *Client) WaitForChannelReady(name string) error {
	namespace := client.Namespace
	channels := client.Eventing.EventingV1alpha1().Channels(namespace)
	if err := base.WaitForChannelState(channels, name, base.IsChannelReady, "ChannelIsReady"); err != nil {
		return err
	}
	return nil
}

// WaitForChannelsReady waits until all channels in the namespace are Ready.
func (client *Client) WaitForChannelsReady() error {
	namespace := client.Namespace
	channels := client.Eventing.EventingV1alpha1().Channels(namespace)
	if err := base.WaitForChannelListState(channels, base.ChannelsReady, "ChannelsAreReady"); err != nil {
		return err
	}
	return nil
}

// WaitForSubscriptionReady waits until the subscription is Ready.
func (client *Client) WaitForSubscriptionReady(name string) error {
	namespace := client.Namespace
	subscriptions := client.Eventing.EventingV1alpha1().Subscriptions(namespace)
	if err := base.WaitForSubscriptionState(subscriptions, name, base.IsSubscriptionReady, "SubscriptionIsReady"); err != nil {
		return err
	}
	return nil
}

// WaitForSubscriptionsReady waits until all subscriptions in the namespace are Ready.
func (client *Client) WaitForSubscriptionsReady() error {
	namespace := client.Namespace
	subscriptions := client.Eventing.EventingV1alpha1().Subscriptions(namespace)
	if err := base.WaitForSubscriptionListState(subscriptions, base.SubscriptionsReady, "SubscriptionsAreReady"); err != nil {
		return err
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
	// FIXME(Fredy-Z): This hacky sleep is added to try mitigating the test flakiness. Will delete it after we find the root cause and fix.
	time.Sleep(10 * time.Second)
	return nil
}
