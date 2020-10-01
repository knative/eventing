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
	cetest "github.com/cloudevents/sdk-go/v2/test"
	uuid "github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"
)

/*
singleEvent tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/

// SingleEventWithKnativeHeaderHelperForChannelTestHelper is the helper function for header_test
func SingleEventWithKnativeHeaderHelperForChannelTestHelper(
	ctx context.Context,
	t *testing.T,
	encoding cloudevents.Encoding,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption,
) {
	channelName := "conformance-headers-channel-" + encoding.String()
	senderName := "conformance-headers-sender-" + encoding.String()
	subscriptionName := "conformance-headers-subscription-" + encoding.String()
	recordEventsPodName := "conformance-headers-recordevents-pod-" + encoding.String()

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		st.Logf("Running header conformance test with channel %q", channel)
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// create channel
		st.Logf("Creating channel")
		client.CreateChannelOrFail(channelName, &channel)

		// create logger service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(recordEventsPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the channel
		eventID := uuid.New().String()
		body := fmt.Sprintf(`{"msg":"TestSingleHeaderEvent %s"}`, eventID)
		event := cloudevents.NewEvent()
		event.SetID(eventID)
		event.SetSource(senderName)
		event.SetType(testlib.DefaultEventType)
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			t.Fatal("Cannot set the payload of the event:", err.Error())
		}

		st.Log("Sending event with knative headers to", senderName)
		client.SendEventToAddressable(
			ctx,
			senderName,
			channelName,
			&channel,
			event,
			sender.WithAdditionalHeaders(map[string]string{"knative-hello": "world"}),
		)

		eventTracker.AssertAtLeast(1, recordevents.AllOf(
			recordevents.MatchEvent(cetest.HasId(eventID)),
			recordevents.HasAdditionalHeader("knative-hello", "world"),
		))

	})
}
