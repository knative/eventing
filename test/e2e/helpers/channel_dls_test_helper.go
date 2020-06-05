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

// ChannelDeadLetterSinkTestHelper is the helper function for channel_deadlettersink_test
func ChannelDeadLetterSinkTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName          = "e2e-channelchain-sender"
		recordEventsPodName = "e2e-channel-dls-recordevents-pod"
	)
	channelNames := []string{"e2e-channel-dls"}
	// subscriptionNames corresponds to Subscriptions
	subscriptionNames := []string{"e2e-channel-dls-subs1"}

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create event logger pod and service as the subscriber
		recordEventsPod := resources.EventRecordPod(recordEventsPodName)
		client.CreatePodOrFail(recordEventsPod, lib.WithService(recordEventsPodName))
		eventTracker, err := client.NewEventInfoStore(recordEventsPodName, t.Logf)
		if err != nil {
			t.Fatalf("Pod tracker failed: %v", err)
		}
		defer eventTracker.Cleanup()

		// create subscriptions that subscribe to a service that does not exist
		client.CreateSubscriptionsOrFail(
			subscriptionNames,
			channelNames[0],
			&channel,
			resources.WithSubscriberForSubscription("does-not-exist"),
			resources.WithDeadLetterSinkForSubscription(recordEventsPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send CloudEvent to the first channel
		event := cloudevents.NewEvent()
		event.SetID("dummy")
		eventSource := fmt.Sprintf("http://%s.svc/", senderName)
		event.SetSource(eventSource)
		event.SetType(lib.DefaultEventType)
		body := fmt.Sprintf(`{"msg":"TestChannelDeadLetterSink %s"}`, uuid.New().String())
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		client.SendEventToAddressable(senderName, channelNames[0], &channel, event)

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames)
		eventTracker.AssertWaitMatchSourceData(t, recordEventsPodName, eventSource, body, expectedContentCount, expectedContentCount)
	})
}
