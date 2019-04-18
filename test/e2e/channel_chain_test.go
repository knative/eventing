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
		senderName    = "e2e-channelchain-sender"
		loggerPodName = "e2e-channelchain-logger-pod"
	)
	channelNames := [2]string{"e2e-channelchain1", "e2e-channelchain2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := [2]string{"e2e-channelchain-subs11", "e2e-channelchain-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := [1]string{"e2e-channelchain-subs21"}

	clients, ns, provisioner, cleaner := Setup(t, true, t.Logf)
	defer TearDown(clients, ns, cleaner, t.Logf)

	// create loggerPod and expose it as a service
	t.Logf("creating logger pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	loggerPod := test.EventLoggerPod(loggerPodName, ns, selector)
	loggerSvc := test.Service(loggerPodName, ns, selector)
	loggerPod, err := CreatePodAndServiceReady(clients, loggerPod, loggerSvc, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create logger pod and service, and get them ready: %v", err)
	}

	// create channels
	t.Logf("Creating Channel and Subscription")
	channels := make([]*v1alpha1.Channel, 0)
	for _, channelName := range channelNames {
		channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(provisioner))
		channels = append(channels, channel)
	}

	// create subscriptions
	subs := make([]*v1alpha1.Subscription, 0)
	// create subscriptions that subscribe the first channel, and reply events directly to the second channel
	for _, subscriptionName := range subscriptionNames1 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[0]), nil, test.ReplyStrategyForChannel(channelNames[1]))
		subs = append(subs, sub)
	}
	// create subscriptions that subscribe the second channel, and call the logging service
	for _, subscriptionName := range subscriptionNames2 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[1]), test.SubscriberSpecForService(loggerPodName), nil)
		subs = append(subs, sub)
	}

	// wait for all channels and subscriptions to become ready
	if err := WithChannelsAndSubscriptionsReady(clients, ns, &channels, &subs, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channel or Subscription were not marked as Ready: %v", err)
	}

	// send fake CloudEvent to the first channel
	body := fmt.Sprintf("TestChannelChainEvent %s", uuid.NewUUID())
	event := &test.CloudEvent{
		Source:   senderName,
		Type:     test.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: test.CloudEventDefaultEncoding,
	}
	if err := SendFakeEventToChannel(clients, event, channels[0], t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channels[0].Name)
	}

	// check if the logging service receives the correct number of event messages
	expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
	if err := WaitForLogContentCount(clients, loggerPodName, loggerPod.Spec.Containers[0].Name, ns, body, expectedContentCount); err != nil {
		t.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", body, expectedContentCount, loggerPodName, err)
	}
}
