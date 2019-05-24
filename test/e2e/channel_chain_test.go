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

	"github.com/knative/eventing/test/base"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestChannelChain tests the following scenario:

EventSource ---> Channel ---> Subscriptions ---> Channel ---> Subscriptions ---> Service(Logger)

*/
func TestChannelChain(t *testing.T) {
	RunTests(t, common.FeatureBasic, testChannelChain)
}

func testChannelChain(t *testing.T, provisioner string) {
	const (
		senderName    = "e2e-channelchain-sender"
		loggerPodName = "e2e-channelchain-logger-pod"
	)
	channelNames := []string{"e2e-channelchain1", "e2e-channelchain2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-channelchain-subs11", "e2e-channelchain-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-channelchain-subs21"}

	client := Setup(t, provisioner, true)
	defer TearDown(client)

	// create channels
	if err := client.CreateChannels(channelNames, provisioner); err != nil {
		t.Fatalf("Failed to create channels %q: %v", channelNames, err)
	}
	client.WaitForChannelsReady()

	// create loggerPod and expose it as a service
	pod := base.EventLoggerPod(loggerPodName)
	if err := client.CreatePod(pod, common.WithService(loggerPodName)); err != nil {
		t.Fatalf("Failed to create logger service: %v", err)
	}

	// create subscriptions that subscribe the first channel, and reply events directly to the second channel
	if err := client.CreateSubscriptions(subscriptionNames1, channelNames[0], base.WithReply(channelNames[1])); err != nil {
		t.Fatalf("Failed to create subscriptions %q for channel %q: %v", subscriptionNames1, channelNames[0], err)
	}
	// create subscriptions that subscribe the second channel, and call the logging service
	if err := client.CreateSubscriptions(subscriptionNames2, channelNames[1], base.WithSubscriberForSubscription(loggerPodName)); err != nil {
		t.Fatalf("Failed to create subscriptions %q for channel %q: %v", subscriptionNames2, channelNames[1], err)
	}

	// wait for all test resources to be ready, so that we can start sending events
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// send fake CloudEvent to the first channel
	body := fmt.Sprintf("TestChannelChainEvent %s", uuid.NewUUID())
	event := &base.CloudEvent{
		Source:   senderName,
		Type:     base.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: base.CloudEventDefaultEncoding,
	}
	if err := client.SendFakeEventToChannel(senderName, channelNames[0], event); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channelNames[0])
	}

	// check if the logging service receives the correct number of event messages
	expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
	if err := client.CheckLog(loggerPodName, common.CheckerContainsCount(body, expectedContentCount)); err != nil {
		t.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", body, expectedContentCount, loggerPodName, err)
	}
}
