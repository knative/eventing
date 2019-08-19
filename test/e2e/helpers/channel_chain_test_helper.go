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

package helpers

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

// ChannelChainTestHelper is the helper function for channel_chain_test
func ChannelChainTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	const (
		senderName    = "e2e-channelchain-sender"
		loggerPodName = "e2e-channelchain-logger-pod"
	)
	channelNames := []string{"e2e-channelchain1", "e2e-channelchain2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-channelchain-subs11", "e2e-channelchain-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-channelchain-subs21"}

	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
		client := common.Setup(st, true)
		defer common.TearDown(client)

		// create channels
		channelTypeMeta := common.GetChannelTypeMeta(channel)
		client.CreateChannelsOrFail(channelNames, channelTypeMeta)
		client.WaitForResourcesReady(channelTypeMeta)

		// create loggerPod and expose it as a service
		pod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(pod, common.WithService(loggerPodName))

		// create subscriptions that subscribe the first channel, and reply events directly to the second channel
		client.CreateSubscriptionsOrFail(
			subscriptionNames1,
			channelNames[0],
			channelTypeMeta,
			resources.WithReplyForSubscription(channelNames[1], channelTypeMeta),
		)
		// create subscriptions that subscribe the second channel, and call the logging service
		client.CreateSubscriptionsOrFail(
			subscriptionNames2,
			channelNames[1],
			channelTypeMeta,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the first channel
		body := fmt.Sprintf("TestChannelChainEvent %s", uuid.NewUUID())
		event := &resources.CloudEvent{
			Source:   senderName,
			Type:     resources.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		if err := client.SendFakeEventToAddressable(senderName, channelNames[0], channelTypeMeta, event); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelNames[0])
		}

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
		if err := client.CheckLog(loggerPodName, common.CheckerContainsCount(body, expectedContentCount)); err != nil {
			st.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", body, expectedContentCount, loggerPodName, err)
		}
	})
}
