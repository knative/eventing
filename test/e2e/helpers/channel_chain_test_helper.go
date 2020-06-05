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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
)

// ChannelChainTestHelper is the helper function for channel_chain_test
func ChannelChainTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName          = "e2e-channelchain-sender"
		recordEventsPodName = "e2e-channelchain-recordevents-pod"
	)
	channelNames := []string{"e2e-channelchain1", "e2e-channelchain2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-channelchain-subs11", "e2e-channelchain-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-channelchain-subs21"}
	eventSource := fmt.Sprintf("http://%s.svc/", senderName)

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create loggerPod and expose it as a service
		recordEventsPod := resources.EventRecordPod(recordEventsPodName)
		client.CreatePodOrFail(recordEventsPod, lib.WithService(recordEventsPodName))
		eventTracker, err := client.NewEventInfoStore(recordEventsPodName, t.Logf)
		if err != nil {
			t.Fatalf("Pod tracker failed: %v", err)
		}
		defer eventTracker.Cleanup()

		// create subscriptions that subscribe the first channel, and reply events directly to the second channel
		client.CreateSubscriptionsOrFail(
			subscriptionNames1,
			channelNames[0],
			&channel,
			resources.WithReplyForSubscription(channelNames[1], &channel),
		)
		// create subscriptions that subscribe the second channel, and call the logging service
		client.CreateSubscriptionsOrFail(
			subscriptionNames2,
			channelNames[1],
			&channel,
			resources.WithSubscriberForSubscription(recordEventsPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send CloudEvent to the first channel
		event := cloudevents.NewEvent()
		event.SetID("dummy")
		event.SetSource(eventSource)
		event.SetType(lib.DefaultEventType)

		body := fmt.Sprintf(`{"msg":"TestSingleEvent %s"}`, uuid.New().String())
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			st.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}

		client.SendEventToAddressable(senderName, channelNames[0], &channel, event)

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)

		// verify the logger service receives the event
		eventTracker.AssertWaitMatchSourceData(t, recordEventsPodName, eventSource, body, expectedContentCount, expectedContentCount)
	})
}
