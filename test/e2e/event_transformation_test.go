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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestEventTransformation tests the following scenario:

             1              2                5                   6             7
EventSource ---> Channel ---> Subscription ---> Channel ---> Subscription ----> Service(Logger)
                                   |  ^
                                 3 |  | 4
                                   |  |
                                   |  ---------
                                   -----------> Service(Transformation)
*/
func TestEventTransformation(t *testing.T) {
	clients, cleaner := Setup(t, t.Logf)

	const (
		senderName = "e2e-complexscen-sender"
		msgPostfix = "######"
	)

	var channelNames = [2]string{"e2e-complexscen1", "e2e-complexscen2"}
	var routeNames = [2]string{"e2e-complexscen-route1", "e2e-complexscen-route2"}
	var subscriptionNames1 = []string{"e2e-complexscen-subs11"}
	var subscriptionNames2 = []string{"e2e-complexscen-subs21", "e2e-complexscen-subs22"}

	// verify namespace
	ns, cleanupNS := CreateNamespaceIfNeeded(t, clients, t.Logf)
	defer cleanupNS()

	// TearDown() needs to be deferred after cleanupNS(). Otherwise the namespace is deleted and all
	// resources in it. So when TearDown() runs, it spews a lot of not found errors.
	defer TearDown(clients, cleaner, t.Logf)

	// create subscriberPods and expose them as services
	t.Logf("creating subscriber pods")

	subscriberPods := make([]*corev1.Pod, 0)
	for i, routeName := range routeNames {
		selector := map[string]string{"e2etest": string(uuid.NewUUID())}
		var subscriberPod *corev1.Pod
		// create the first subscriber for the event transformation, and the second for the logging
		if i == 0 {
			subscriberPod = test.EventTransformationPod(routeName, ns, selector, msgPostfix)
		} else if i == 1 {
			subscriberPod = test.EventLoggerPod(routeName, ns, selector)
		}

		subscriberPod, err := CreatePodAndServiceReady(clients, subscriberPod, routeName, ns, selector, t.Logf, cleaner)
		if err != nil {
			t.Fatalf("Failed to create subscriber pod and service, and get them ready: %v", err)
		}

		subscriberPods = append(subscriberPods, subscriberPod)
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
	// create subscriptions that subscribe the first channel, use the transformation service to transform the events and then forward the transformed events to the second channel
	for _, subscriptionName := range subscriptionNames1 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[0]), test.SubscriberSpecForService(routeNames[0]), test.ReplyStrategyForChannel(channelNames[1]))
		t.Logf("sub: %#v", sub)
		subs = append(subs, sub)
	}
	// create subscriptions that subscribe the second channel, and call the logging service
	for _, subscriptionName := range subscriptionNames2 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[1]), test.SubscriberSpecForService(routeNames[1]), nil)
		t.Logf("sub: %#v", sub)
		subs = append(subs, sub)
	}

	// wait for all channels and subscriptions to become ready
	if err := WithChannelsAndSubscriptionsReady(clients, &channels, &subs, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channels or Subscription were not marked as Ready: %v", err)
	}

	// send fake CloudEvent to the first channel
	body := fmt.Sprintf("TestEventTransformation %s", uuid.NewUUID())
	SendFakeEventToChannel(clients, senderName, body, test.CloudEventDefaultType, test.CloudEventDefaultEncoding, channels[0], ns, t.Logf, cleaner)

	// check if the logging service receives the correct number of event messages
	if err := WaitForLogContentCount(clients, subscriberPods[1].Name, subscriberPods[1].Spec.Containers[0].Name, body+msgPostfix, len(subscriptionNames1)*len(subscriptionNames2)); err != nil {
		t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body+msgPostfix, subscriberPods[1].Name, err)
	}
}
