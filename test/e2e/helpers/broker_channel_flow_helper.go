/*
Copyright 2020 The Knative Authors
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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

/*
BrokerChannelFlowWithTransformation tests the following topology:

                   ------------- ----------------------
                   |           | |                    |
                   v	       | v                    |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   |
                   |-------> Trigger2 -------> Service(Logger1)
                   |
                   |
                   |-------> Trigger3 -------> Channel --------> Subscription --------> Service(Logger2)

Explanation:
Trigger1 filters the orignal event and transforms it to a new event,
Trigger2 logs all events,
Trigger3 filters the transformed event and sends it to Channel.

*/
func BrokerChannelFlowWithTransformation(
	ctx context.Context,
	t *testing.T,
	brokerClass string,
	brokerVersion string,
	triggerVersion string,
	channelTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderName = "e2e-brokerchannel-sender"
		brokerName = "e2e-brokerchannel-broker"

		any                    = v1.TriggerAnyFilter
		eventType              = "type1"
		transformedEventType   = "type2"
		eventSource            = "http://source1.com"
		transformedEventSource = "http://source2.com"
		eventBody              = `{"msg":"e2e-brokerchannel-body"}`
		transformedBody        = `{"msg":"transformed body"}`

		triggerName1 = "e2e-brokerchannel-trigger1"
		triggerName2 = "e2e-brokerchannel-trigger2"
		triggerName3 = "e2e-brokerchannel-trigger3"

		transformationPodName            = "e2e-brokerchannel-trans-pod"
		allEventsRecorderPodName         = "e2e-brokerchannel-logger-pod1"
		transformedEventsRecorderPodName = "e2e-brokerchannel-logger-pod2"

		channelName      = "e2e-brokerchannel-channel"
		subscriptionName = "e2e-brokerchannel-subscription"
	)

	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		config := client.CreateBrokerConfigMapOrFail(brokerName, &channel)
		//&channel

		// create a new broker
		client.CreateBrokerOrFail(brokerName, resources.WithBrokerClassForBroker(brokerClass), resources.WithConfigForBroker(config))
		client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

		// eventToSend is the event sent as input of the test
		eventToSend := cloudevents.NewEvent()
		eventToSend.SetID(uuid.New().String())
		eventToSend.SetType(eventType)
		eventToSend.SetSource(eventSource)
		if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
			t.Fatal("Cannot set the payload of the event:", err.Error())
		}

		// create the transformation service for trigger1
		recordevents.DeployEventRecordOrFail(
			ctx,
			client,
			transformationPodName,
			recordevents.ReplyWithTransformedEvent(
				transformedEventType,
				transformedEventSource,
				transformedBody,
			),
		)

		// create trigger1 to receive the original event, and do event transformation
		client.CreateTriggerOrFail(
			triggerName1,
			resources.WithBroker(brokerName),
			resources.WithAttributesTriggerFilter(eventSource, eventType, nil),
			resources.WithSubscriberServiceRefForTrigger(transformationPodName),
		)
		// create event tracker that should receive all sent events
		allEventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, allEventsRecorderPodName)

		// create trigger to receive all the events
		client.CreateTriggerOrFail(
			triggerName2,
			resources.WithBroker(brokerName),
			resources.WithAttributesTriggerFilter(any, any, nil),
			resources.WithSubscriberServiceRefForTrigger(allEventsRecorderPodName),
		)
		// create channel for trigger3
		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		// create trigger3 to receive the transformed event, and send it to the channel
		channelURL, err := client.GetAddressableURI(channelName, &channel)
		if err != nil {
			st.Fatalf("Failed to get the url for the channel %q: %+v", channelName, err)
		}
		client.CreateTriggerOrFail(
			triggerName3,
			resources.WithBroker(brokerName),
			resources.WithAttributesTriggerFilter(transformedEventSource, transformedEventType, nil),
			resources.WithSubscriberURIForTrigger(channelURL),
		)

		// create event tracker that should receive only transformed events
		transformedEventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, transformedEventsRecorderPodName)

		// create subscription
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(transformedEventsRecorderPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// send CloudEvent to the broker
		client.SendEventToAddressable(ctx, senderName, brokerName, testlib.BrokerTypeMeta, eventToSend)

		// Assert the results on the event trackers
		originalEventMatcher := recordevents.MatchEvent(AllOf(
			HasSource(eventSource),
			HasType(eventType),
			HasData([]byte(eventBody)),
		))
		transformedEventMatcher := recordevents.MatchEvent(AllOf(
			HasSource(transformedEventSource),
			HasType(transformedEventType),
			HasData([]byte(transformedBody)),
		))

		allEventTracker.AssertAtLeast(1, originalEventMatcher)
		allEventTracker.AssertAtLeast(1, transformedEventMatcher)

		transformedEventTracker.AssertAtLeast(1, transformedEventMatcher)
		transformedEventTracker.AssertNot(originalEventMatcher)
	})
}
