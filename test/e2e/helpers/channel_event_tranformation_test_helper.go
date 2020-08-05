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
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// EventTransformationForSubscriptionTestHelper is the helper function for channel_event_tranformation_test
func EventTransformationForSubscriptionTestHelper(t *testing.T,
	subscriptionVersion SubscriptionVersion,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	senderName := "e2e-eventtransformation-sender"
	channelNames := []string{"e2e-eventtransformation1", "e2e-eventtransformation2"}
	eventSource := fmt.Sprintf("http://%s.svc/", senderName)
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-eventtransformation-subs11", "e2e-eventtransformation-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-eventtransformation-subs21", "e2e-eventtransformation-subs22"}
	transformationPodName := "e2e-eventtransformation-transformation-pod"
	recordEventsPodName := "e2e-eventtransformation-recordevents-pod"

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create transformation pod and service
		eventAfterTransformation := cloudevents.NewEvent()
		eventAfterTransformation.SetID("dummy-transformed")
		eventAfterTransformation.SetSource(eventSource)
		eventAfterTransformation.SetType(testlib.DefaultEventType)
		transformedEventBody := fmt.Sprintf(`{"msg":"eventBody %s"}`, uuid.New().String())
		if err := eventAfterTransformation.SetData(cloudevents.ApplicationJSON, []byte(transformedEventBody)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		transformationPod := resources.EventTransformationPod(
			transformationPodName,
			eventAfterTransformation.Type(),
			eventAfterTransformation.Source(),
			eventAfterTransformation.Data(),
		)
		client.CreatePodOrFail(transformationPod, testlib.WithService(transformationPodName))

		// create event logger pod and service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(client, recordEventsPodName)
		switch subscriptionVersion {
		case SubscriptionV1:
			// create subscriptions that subscribe the first channel, use the transformation service to transform the events and then forward the transformed events to the second channel
			client.CreateSubscriptionsV1OrFail(
				subscriptionNames1,
				channelNames[0],
				&channel,
				resources.WithSubscriberForSubscriptionV1(transformationPodName),
				resources.WithReplyForSubscriptionV1(channelNames[1], &channel),
			)
			// create subscriptions that subscribe the second channel, and forward the received events to the logger service
			client.CreateSubscriptionsV1OrFail(
				subscriptionNames2,
				channelNames[1],
				&channel,
				resources.WithSubscriberForSubscriptionV1(recordEventsPodName),
			)
		case SubscriptionV1beta1:
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
				resources.WithSubscriberForSubscription(recordEventsPodName),
			)

		default:
			t.Fatalf("Invalid subscription version")
		}

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send  CloudEvent to the first channel
		eventToSend := cloudevents.NewEvent()
		eventToSend.SetID("dummy")
		eventToSend.SetSource(eventSource)
		eventToSend.SetType(testlib.DefaultEventType)
		eventBody := fmt.Sprintf(`{"msg":"TestEventTransformation %s"}`, uuid.New().String())
		if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		client.SendEventToAddressable(senderName, channelNames[0], &channel, eventToSend)

		// check if the logging service receives the correct number of event messages
		eventTracker.AssertAtLeast(len(subscriptionNames1)*len(subscriptionNames2), recordevents.MatchEvent(
			HasSource(eventSource),
			HasData([]byte(transformedEventBody)),
		))
	})
}
