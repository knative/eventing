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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// EventTransformationForSubscriptionTestHelper is the helper function for channel_event_tranformation_test
func EventTransformationForSubscriptionTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	senderName := "e2e-eventtransformation-sender"
	channelNames := []string{"e2e-eventtransformation1", "e2e-eventtransformation2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-eventtransformation-subs11", "e2e-eventtransformation-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-eventtransformation-subs21", "e2e-eventtransformation-subs22"}
	transformationPodName := "e2e-eventtransformation-transformation-pod"
	loggerPodName := "e2e-eventtransformation-logger-pod"

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create transformation pod and service
		transformedEventBody := fmt.Sprintf("eventBody %s", uuid.NewUUID())
		eventAfterTransformation := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
			cloudevents.WithSource(senderName),
		)
		transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
		client.CreatePodOrFail(transformationPod, lib.WithService(transformationPodName))

		// create logger pod and service
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		// create subscriptions that subscribe the first channel, use the transformation service to transform the events and then forward the transformed events to the second channel
		client.CreateSubscriptionsOrFail(
			subscriptionNames1,
			channelNames[0],
			&channel,
			resources.WithSubscriberForSubscription(transformationPodName),
			resources.WithReplyForSubscription(channelNames[1], &channel),
		)
		// create subscriptions that subscribe the second channel, and forward the received events to the logger service
		client.CreateSubscriptionsOrFail(
			subscriptionNames2,
			channelNames[1],
			&channel,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the first channel
		eventBody := fmt.Sprintf("TestEventTransformation %s", uuid.NewUUID())
		eventToSend := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, eventBody),
			cloudevents.WithSource(senderName),
		)
		client.SendFakeEventToAddressableOrFail(senderName, channelNames[0], &channel, eventToSend)

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
		if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(transformedEventBody, expectedContentCount)); err != nil {
			st.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", transformedEventBody, expectedContentCount, loggerPodName, err)
		}
	})
}
