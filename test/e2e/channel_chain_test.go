// +build e2e

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

package e2e

import (
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestChannelChain tests the following scenario:

EventSource ---> Channel ---> Subscriptions ---> Channel ---> Subscriptions ---> Service(Logger)

*/
func TestChannelChain(t *testing.T) {
	const (
		senderName = "e2e-channelchain-sender"
		routeName  = "e2e-channelchain-route"
	)
	var channelNames = [2]string{"e2e-channelchain1", "e2e-channelchain2"}
	var subscriptionNames1 = [2]string{"e2e-complexscen-subs11", "e2e-complexscen-subs12"}
	var subscriptionNames2 = [1]string{"e2e-complexscen-subs21"}

	clients, cleaner := Setup(t, t.Logf)
	// verify namespace
	ns, cleanupNS := CreateNamespaceIfNeeded(t, clients, t.Logf)
	defer cleanupNS()

	// TearDown() needs to be deferred after cleanupNS(). Otherwise the namespace is deleted and all
	// resources in it. So when TearDown() runs, it spews a lot of not found errors.
	defer TearDown(clients, cleaner, t.Logf)

	// create subscriberPod and expose it as a service
	t.Logf("creating subscriber pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	subscriberPod := test.EventLoggerPod(routeName, ns, selector)
	subscriberPod, err := CreatePodAndServiceReady(clients, subscriberPod, routeName, ns, selector, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create subscriber pod and service, and get them ready: %v", err)
	}

	// create channels
	t.Logf("Creating Channel and Subscription")
	if test.EventingFlags.Provisioner == "" {
		t.Fatal("ClusterChannelProvisioner must be set to a non-empty string. Either do not specify --clusterChannelProvisioner or set to something other than the empty string")
	}
	channels := make([]*v1alpha1.Channel, 0)
	for _, channelName := range channelNames {
		channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(test.EventingFlags.Provisioner))
		t.Logf("channel: %#v", channel)
		channels = append(channels, channel)
	}

	// create subscriptions
	subs := make([]*v1alpha1.Subscription, 0)
	// create subscriptions that subscribe the first channel, and reply events directly to the second channel
	for _, subscriptionName := range subscriptionNames1 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[0]), nil, test.ReplyStrategyForChannel(channelNames[1]))
		t.Logf("sub: %#v", sub)
		subs = append(subs, sub)
	}
	// create subscriptions that subscribe the second channel, and call the logging service
	for _, subscriptionName := range subscriptionNames2 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[1]), test.SubscriberSpecForService(routeName), nil)
		t.Logf("sub: %#v", sub)
		subs = append(subs, sub)
	}

	// wait for all channels and subscriptions to become ready
	if err := WithChannelsAndSubscriptionsReady(clients, &channels, &subs, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channel or Subscription were not marked as Ready: %v", err)
	}

	// send fake CloudEvent to the first channel
	body := fmt.Sprintf("TestChannelChainEvent %s", uuid.NewUUID())
	if err := SendFakeEventToChannel(clients, senderName, body, test.CloudEventDefaultType, test.CloudEventDefaultEncoding, channels[0], ns, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channels[0].Name)
	}

	// check if the logging service receives the correct number of event messages
	expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
	if err := WaitForLogContentCount(clients, routeName, subscriberPod.Spec.Containers[0].Name, body, expectedContentCount); err != nil {
		t.Fatalf("String %q does not appear %d times in logs of subscriber pod %q: %v", body, expectedContentCount, subscriberPod.Name, err)
	}
}
