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
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	duckv1 "knative.dev/pkg/apis/duck/v1"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// ChannelDeadLetterSinkTestHelper is the helper function for channel_deadlettersink_test
// Deprecated, use reconciler-test based tests.
func ChannelDeadLetterSinkTestHelper(
	ctx context.Context,
	t *testing.T,
	subscriptionVersion SubscriptionVersion,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderName          = "e2e-channelchain-sender"
		recordEventsPodName = "e2e-channel-dls-recordevents-pod"
	)
	channelNames := []string{"e2e-channel-dls"}
	// subscriptionNames corresponds to Subscriptions
	subscriptionNames := []string{"e2e-channel-dls-subs1"}

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create event logger pod and service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscriptions that subscribe to a service that does not exist
		switch subscriptionVersion {
		case SubscriptionV1:
			client.CreateSubscriptionsOrFail(
				subscriptionNames,
				channelNames[0],
				&channel,
				resources.WithSubscriberForSubscription("does-not-exist"),
				resources.WithDeadLetterSinkForSubscription(recordEventsPodName),
			)
		default:
			t.Fatalf("Invalid subscription version")
		}

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the first channel
		event := cloudevents.NewEvent()
		event.SetID("test")
		eventSource := fmt.Sprintf("http://%s.svc/", senderName)
		event.SetSource(eventSource)
		event.SetType(testlib.DefaultEventType)
		body := fmt.Sprintf(`{"msg":"TestChannelDeadLetterSink %s"}`, uuid.New().String())
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			t.Fatal("Cannot set the payload of the event:", err.Error())
		}
		client.SendEventToAddressable(ctx, senderName, channelNames[0], &channel, event)

		// check if the logging service receives the correct number of event messages
		eventTracker.AssertAtLeast(len(subscriptionNames), recordevents.MatchEvent(
			HasSource(eventSource),
			HasData([]byte(body)),
		))
	})
}

// ChannelDeadLetterDefaultSinkTestHelper is the helper function for channel_deadlettersink_test, but setting the delivery from the channel spec
// Deprecated, use reconciler-test based tests.
func ChannelDeadLetterSinkDefaultTestHelper(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderName          = "e2e-channelchain-sender"
		recordEventsPodName = "e2e-channel-dls-recordevents-pod"
		channelName         = "e2e-channel-dls"
	)
	channelGK := messagingv1.SchemeGroupVersion.WithKind("Channel").GroupKind()
	// subscriptionNames corresponds to Subscriptions
	subscriptionNames := []string{"e2e-channel-dls-subs1"}

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		thisChannelGk := channel.GroupVersionKind().GroupKind()
		if equality.Semantic.DeepEqual(thisChannelGk, channelGK) {
			st.Skip("It doesn't make sense to create a messaging.Channel with a backing messaging.Channel")
			return
		}

		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// create channel
		client.CreateChannelWithDefaultOrFail(&messagingv1.Channel{
			ObjectMeta: metav1.ObjectMeta{
				Name:      channelName,
				Namespace: client.Namespace,
			},
			Spec: messagingv1.ChannelSpec{
				ChannelTemplate: &messagingv1.ChannelTemplateSpec{
					TypeMeta: channel,
				},
				ChannelableSpec: eventingduckv1.ChannelableSpec{
					Delivery: &eventingduckv1.DeliverySpec{
						DeadLetterSink: &duckv1.Destination{
							Ref: resources.KnativeRefForService(recordEventsPodName, client.Namespace),
						},
						Retry: pointer.Int32Ptr(10),
					},
				},
			},
		})
		client.WaitForResourcesReadyOrFail(&channel)

		// create event logger pod and service as the subscriber
		eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
		// create subscriptions that subscribe to a service that does not exist
		client.CreateSubscriptionsOrFail(
			subscriptionNames,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription("does-not-exist"),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the first channel
		event := cloudevents.NewEvent()
		event.SetID("test")
		eventSource := fmt.Sprintf("http://%s.svc/", senderName)
		event.SetSource(eventSource)
		event.SetType(testlib.DefaultEventType)
		body := fmt.Sprintf(`{"msg":"TestChannelDeadLetterSink %s"}`, uuid.New().String())
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
			t.Fatal("Cannot set the payload of the event:", err.Error())
		}
		client.SendEventToAddressable(ctx, senderName, channelName, &channel, event)

		// check if the logging service receives the correct number of event messages
		eventTracker.AssertAtLeast(len(subscriptionNames), recordevents.MatchEvent(
			HasSource(eventSource),
			HasData([]byte(body)),
		))
	})
}
