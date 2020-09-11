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
	"context"
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// ChannelChainTestHelper is the helper function for channel_chain_test
func ChannelChainTestHelper(
	ctx context.Context,
	t *testing.T,
	subscriptionVersion SubscriptionVersion,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
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

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create loggerPod and expose it as a service
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscription to subscribe the channel, and forward the received events to the logger service
		switch subscriptionVersion {
		case SubscriptionV1:
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
		case SubscriptionV1beta1:
			// create subscriptions that subscribe the first channel, and reply events directly to the second channel
			client.CreateSubscriptionsV1OrFail(
				subscriptionNames1,
				channelNames[0],
				&channel,
				resources.WithReplyForSubscriptionV1(channelNames[1], &channel),
			)
			// create subscriptions that subscribe the second channel, and call the logging service
			client.CreateSubscriptionsV1OrFail(
				subscriptionNames2,
				channelNames[1],
				&channel,
				resources.WithSubscriberForSubscriptionV1(recordEventsPodName),
			)

		default:
			t.Fatalf("Invalid subscription version")
		}
		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the first channel
		event := cloudevents.NewEvent()
		event.SetID("dummy")
		event.SetSource(eventSource)
		event.SetType(testlib.DefaultEventType)

		body := fmt.Sprintf(`{"msg":"TestSingleEvent %s"}`, uuid.New().String())
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			st.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}

		client.SendEventToAddressable(ctx, senderName, channelNames[0], &channel, event)

		// verify the logger service receives the event
		eventTracker.AssertAtLeast(len(subscriptionNames1)*len(subscriptionNames2), recordevents.MatchEvent(
			HasSource(eventSource),
			HasData([]byte(body)),
		))
	})
}
