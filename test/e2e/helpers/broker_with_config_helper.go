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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

// TestBrokerWithConfig is the helper function for broker_with_config_test
func TestBrokerWithConfig(t *testing.T,
	brokerClass string,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName = "e2e-brokerchannel-sender"
		brokerName = "e2e-brokerchannel-broker"

		any                    = v1beta1.TriggerAnyFilter
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

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		config := client.CreateBrokerConfigMapOrFail(brokerName, &channel)
		//&channel

		// create a new broker
		client.CreateBrokerV1Beta1OrFail(brokerName, resources.WithBrokerClassForBrokerV1Beta1(brokerClass), resources.WithConfigForBrokerV1Beta1(config))
		client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)

		// eventToSend is the event sent as input of the test
		eventToSend := cloudevents.NewEvent()
		eventToSend.SetID(uuid.New().String())
		eventToSend.SetType(eventType)
		eventToSend.SetSource(eventSource)
		if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}

		// create the transformation service for trigger1
		transformationPod := resources.EventTransformationPod(
			transformationPodName,
			transformedEventType,
			transformedEventSource,
			[]byte(transformedBody),
		)
		client.CreatePodOrFail(transformationPod, lib.WithService(transformationPodName))

		// create trigger1 to receive the original event, and do event transformation
		client.CreateTriggerOrFailV1Beta1(
			triggerName1,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(eventSource, eventType, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(transformationPodName),
		)

		// create event tracker that should receive all sent events
		allEventTracker, _ := recordevents.StartEventRecordOrFail(client, allEventsRecorderPodName)
		defer allEventTracker.Cleanup()

		// create trigger to receive all the events
		client.CreateTriggerOrFailV1Beta1(
			triggerName2,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(any, any, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(allEventsRecorderPodName),
		)

		// create channel for trigger3
		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		// create trigger3 to receive the transformed event, and send it to the channel
		channelURL, err := client.GetAddressableURI(channelName, &channel)
		if err != nil {
			st.Fatalf("Failed to get the url for the channel %q: %v", channelName, err)
		}
		client.CreateTriggerOrFailV1Beta1(
			triggerName3,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(transformedEventSource, transformedEventType, nil),
			resources.WithSubscriberURIForTriggerV1Beta1(channelURL),
		)

		// create event tracker that should receive only transformed events
		transformedEventTracker, _ := recordevents.StartEventRecordOrFail(client, transformedEventsRecorderPodName)
		defer transformedEventTracker.Cleanup()

		// create subscription
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(transformedEventsRecorderPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send CloudEvent to the broker
		client.SendEventToAddressable(senderName, brokerName, lib.BrokerTypeMeta, eventToSend)

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
