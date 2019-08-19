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
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

// BrokerChannelFlowTestHelper is the helper function for broker_channel_flow_test
func BrokerChannelFlowTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	const (
		senderName = "e2e-brokerchannel-sender"
		brokerName = "e2e-brokerchannel-broker"

		any          = v1alpha1.TriggerAnyFilter
		eventType1   = "type1"
		eventType2   = "type2"
		eventSource1 = "source1"
		eventSource2 = "source2"
		eventBody    = "e2e-brokerchannel-body"

		triggerName1 = "e2e-brokerchannel-trigger1"
		triggerName2 = "e2e-brokerchannel-trigger2"
		triggerName3 = "e2e-brokerchannel-trigger3"

		transformationPodName = "e2e-brokerchannel-trans-pod"
		loggerPodName1        = "e2e-brokerchannel-logger-pod1"
		loggerPodName2        = "e2e-brokerchannel-logger-pod2"

		channelName      = "e2e-brokerchannel-channel"
		subscriptionName = "e2e-brokerchannel-subscription"
	)

	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
		client := common.Setup(st, true)
		defer common.TearDown(client)
		channelTypeMeta := common.GetChannelTypeMeta(channel)

		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateRBACResourcesForBrokers()

		// create a new broker
		client.CreateBrokerOrFail(brokerName, channelTypeMeta)
		client.WaitForResourceReady(brokerName, common.BrokerTypeMeta)

		// create the event we want to transform to
		transformedEventBody := fmt.Sprintf("%s %s", eventBody, string(uuid.NewUUID()))
		eventAfterTransformation := &resources.CloudEvent{
			Source:   eventSource2,
			Type:     eventType2,
			Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
			Encoding: resources.CloudEventDefaultEncoding,
		}

		// create the transformation service for trigger1
		transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
		client.CreatePodOrFail(transformationPod, common.WithService(transformationPodName))

		// create trigger1 to receive the original event, and do event transformation
		client.CreateTriggerOrFail(
			triggerName1,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource1, eventType1),
			resources.WithSubscriberRefForTrigger(transformationPodName),
		)

		// create logger pod and service for trigger2
		loggerPod1 := resources.EventLoggerPod(loggerPodName1)
		client.CreatePodOrFail(loggerPod1, common.WithService(loggerPodName1))

		// create trigger2 to receive all the events
		client.CreateTriggerOrFail(
			triggerName2,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(any, any),
			resources.WithSubscriberRefForTrigger(loggerPodName1),
		)

		// create channel for trigger3
		client.CreateChannelOrFail(channelName, channelTypeMeta)
		client.WaitForResourceReady(channelName, channelTypeMeta)

		// create trigger3 to receive the transformed event, and send it to the channel
		channelURL, err := client.GetAddressableURI(channelName, channelTypeMeta)
		if err != nil {
			st.Fatalf("Failed to get the url for the channel %q: %v", channelName, err)
		}
		client.CreateTriggerOrFail(
			triggerName3,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource2, eventType2),
			resources.WithSubscriberURIForTrigger(channelURL),
		)

		// create logger pod and service for subscription
		loggerPod2 := resources.EventLoggerPod(loggerPodName2)
		client.CreatePodOrFail(loggerPod2, common.WithService(loggerPodName2))

		// create subscription
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			channelTypeMeta,
			resources.WithSubscriberForSubscription(loggerPodName2),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the broker
		eventToSend := &resources.CloudEvent{
			Source:   eventSource1,
			Type:     eventType1,
			Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		if err := client.SendFakeEventToAddressable(senderName, brokerName, common.BrokerTypeMeta, eventToSend); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
		}

		// check if trigger2's logging service receives both events
		eventBodies := []string{transformedEventBody, eventBody}
		if err := client.CheckLog(loggerPodName1, common.CheckerContainsAll([]string{transformedEventBody, eventBody})); err != nil {
			st.Fatalf("Strings %v not found in logs of logger pod %q: %v", eventBodies, loggerPodName1, err)
		}

		// check if subscription's logging service receives the transformed event
		if err := client.CheckLog(loggerPodName2, common.CheckerContains(transformedEventBody)); err != nil {
			st.Fatalf("Strings %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName2, err)
		}
	})
}
