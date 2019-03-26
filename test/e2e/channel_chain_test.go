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
	pkgTest "github.com/knative/pkg/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestChannelChain(t *testing.T) {
	const (
		senderName = "e2e-channelchain-sender"
		routeName  = "e2e-channelchain-route"
	)
	var channelNames = [2]string{"e2e-channelchain1", "e2e-channelchain2"}
	var subscriptionNames1 = CreateRandomSubscriptionNames("e2e-complexscen-subs1")
	var subscriptionNames2 = CreateRandomSubscriptionNames("e2e-complexscen-subs2")

	clients, cleaner := Setup(t, t.Logf)

	// verify namespace
	ns, cleanupNS := NamespaceExists(t, clients, t.Logf)
	defer cleanupNS()

	// TearDown() needs to be deferred after cleanupNS(). Otherwise the namespace is deleted and all
	// resources in it. So when TearDown() runs, it spews a lot of not found errors.
	defer TearDown(clients, cleaner, t.Logf)

	// create subscriberPod and expose it as a service
	t.Logf("creating subscriber pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	subscriberPod := test.EventLoggerPod(routeName, ns, selector)
	if err := CreatePod(clients, subscriberPod, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to create event logger pod: %v", err)
	}
	if err := pkgTest.WaitForAllPodsRunning(clients.Kube, ns); err != nil {
		t.Fatalf("Error waiting for logger pod to become running: %v", err)
	}
	t.Logf("subscriber pod running")

	subscriberSvc := test.Service(routeName, ns, selector)
	if err := CreateService(clients, subscriberSvc, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to create event logger service: %v", err)
	}

	// Reload subscriberPod to get IP
	subscriberPod, err := clients.Kube.Kube.CoreV1().Pods(subscriberPod.Namespace).Get(subscriberPod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get subscriber pod: %v", err)
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

	// create sender pod
	t.Logf("Creating event sender")
	body := fmt.Sprintf("TestChannelChainEvent %s", uuid.NewUUID())
	event := test.CloudEvent{
		Source:   senderName,
		Type:     "test.eventing.knative.dev",
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: test.CloudEventEncodingBinary,
	}
	url := fmt.Sprintf("http://%s", channels[0].Status.Address.Hostname)
	pod := test.EventSenderPod(senderName, ns, url, event)
	t.Logf("sender pod: %#v", pod)
	if err := CreatePod(clients, pod, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to create event sender pod: %v", err)
	}

	// check if the logging service receives the correct number of event messages
	if err := WaitForLogContentCount(clients, routeName, subscriberPod.Spec.Containers[0].Name, body, len(subscriptionNames1)*len(subscriptionNames2)); err != nil {
		t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body, routeName, err)
	}
}
