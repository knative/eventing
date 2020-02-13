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

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// BrokerChannelFlowTestHelper is the helper function for broker_channel_flow_test
func BrokerChannelFlowTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
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

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateRBACResourcesForBrokers()

		// create a new broker
		client.CreateBrokerOrFail(brokerName, resources.WithChannelTemplateForBroker(&channel))
		client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)

		// create the event we want to transform to
		transformedEventBody := fmt.Sprintf("%s %s", eventBody, string(uuid.NewUUID()))
		eventAfterTransformation := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
			cloudevents.WithSource(eventSource2),
			cloudevents.WithType(eventType2),
		)

		// create the transformation service for trigger1
		transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
		client.CreatePodOrFail(transformationPod, lib.WithService(transformationPodName))

		// create trigger1 to receive the original event, and do event transformation
		client.CreateTriggerOrFail(
			triggerName1,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource1, eventType1),
			resources.WithSubscriberServiceRefForTrigger(transformationPodName),
		)

		// create logger pod and service for trigger2
		loggerPod1 := resources.EventLoggerPod(loggerPodName1)
		client.CreatePodOrFail(loggerPod1, lib.WithService(loggerPodName1))

		// create trigger2 to receive all the events
		client.CreateTriggerOrFail(
			triggerName2,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(any, any),
			resources.WithSubscriberServiceRefForTrigger(loggerPodName1),
		)

		// create channel for trigger3
		client.CreateChannelOrFail(channelName, &channel)
		client.WaitForResourceReadyOrFail(channelName, &channel)

		// create trigger3 to receive the transformed event, and send it to the channel
		channelURL, err := client.GetAddressableURI(channelName, &channel)
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
		client.CreatePodOrFail(loggerPod2, lib.WithService(loggerPodName2))

		// create subscription
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(loggerPodName2),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the broker
		eventToSend := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, eventBody),
			cloudevents.WithSource(eventSource1),
			cloudevents.WithType(eventType1),
		)
		client.SendFakeEventToAddressableOrFail(senderName, brokerName, lib.BrokerTypeMeta, eventToSend)

		// check if trigger2's logging service receives both events
		eventBodies := []string{transformedEventBody, eventBody}
		if err := client.CheckLog(loggerPodName1, lib.CheckerContainsAll([]string{transformedEventBody, eventBody})); err != nil {
			st.Fatalf("Strings %v not found in logs of logger pod %q: %v", eventBodies, loggerPodName1, err)
		}

		// check if subscription's logging service receives the transformed event
		if err := client.CheckLog(loggerPodName2, lib.CheckerContains(transformedEventBody)); err != nil {
			st.Fatalf("Strings %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName2, err)
		}
	})
}
