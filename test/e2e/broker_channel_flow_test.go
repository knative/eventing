// +build e2e

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

package e2e

import (
	"fmt"
	"testing"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/test/base"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestEventTransformationForTrigger tests the following topology:

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
Trigger1 filters the orignal event and tranforms it to a new event,
Trigger2 logs all events,
Trigger3 filters the transformed event and sends it to Channel.

*/
func TestBrokerChannelFlow(t *testing.T) {
	RunTests(t, common.FeatureBasic, testBrokerChannelFlow)
}

func testBrokerChannelFlow(t *testing.T, provisioner string) {
	const (
		senderName    = "e2e-brokerchannel-sender"
		brokerName    = "e2e-brokerchannel-broker"
		saIngressName = "eventing-broker-ingress"
		saFilterName  = "eventing-broker-filter"

		// This ClusterRole is installed in Knative Eventing setup, see https://github.com/knative/eventing/tree/master/docs/broker#manual-setup.
		crIngressName = "eventing-broker-ingress"
		crFilterName  = "eventing-broker-filter"

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

	client := Setup(t, provisioner, true)
	defer TearDown(client)

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	if err := client.CreateServiceAccountAndBinding(saIngressName, crIngressName); err != nil {
		t.Fatalf("Failed to create the Ingress ServiceAccount and ServiceAccountRoleBinding: %v", err)
	}
	if err := client.CreateServiceAccountAndBinding(saFilterName, crFilterName); err != nil {
		t.Fatalf("Failed to create the Filter ServiceAccount and ServiceAccountRoleBinding: %v", err)
	}

	// create a new broker
	if err := client.CreateBroker(brokerName, provisioner); err != nil {
		t.Fatalf("Failed to create the Broker: %q, %v", brokerName, err)
	}
	client.WaitForBrokerReady(brokerName)

	// create the event we want to transform to
	transformedEventBody := fmt.Sprintf("%s %s", eventBody, string(uuid.NewUUID()))
	eventAfterTransformation := &base.CloudEvent{
		Source:   eventSource2,
		Type:     eventType2,
		Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
		Encoding: base.CloudEventDefaultEncoding,
	}

	// create the transformation service for trigger1
	transformationPod := base.EventTransformationPod(transformationPodName, eventAfterTransformation)
	if err := client.CreatePod(transformationPod, common.WithService(transformationPodName)); err != nil {
		t.Fatalf("Failed to create transformation service %q: %v", transformationPodName, err)
	}

	// create trigger1 to receive the original event, and do event transformation
	if err := client.CreateTrigger(
		triggerName1,
		base.WithBroker(brokerName),
		base.WithTriggerFilter(eventSource1, eventType1),
		base.WithSubscriberRefForTrigger(transformationPodName),
	); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName1, err)
	}

	// create logger pod and service for trigger2
	loggerPod1 := base.EventLoggerPod(loggerPodName1)
	if err := client.CreatePod(loggerPod1, common.WithService(loggerPodName1)); err != nil {
		t.Fatalf("Failed to create logger service %q: %v", loggerPodName1, err)
	}

	// create trigger2 to receive all the events
	if err := client.CreateTrigger(
		triggerName2,
		base.WithBroker(brokerName),
		base.WithTriggerFilter(any, any),
		base.WithSubscriberRefForTrigger(loggerPodName1),
	); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName2, err)
	}

	// create channel for trigger3
	if err := client.CreateChannel(channelName, provisioner); err != nil {
		t.Fatalf("Failed to create channel %q: %v", channelName, err)
	}
	client.WaitForChannelReady(channelName)

	// create trigger3 to receive the transformed event, and send it to the channel
	channelURL, err := client.GetChannelURL(channelName)
	if err != nil {
		t.Fatalf("Failed to get the url for the channel %q: %v", channelName, err)
	}
	if err := client.CreateTrigger(
		triggerName3,
		base.WithBroker(brokerName),
		base.WithTriggerFilter(eventSource2, eventType2),
		base.WithSubscriberURIForTrigger(channelURL),
	); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName3, err)
	}

	// create logger pod and service for subscription
	loggerPod2 := base.EventLoggerPod(loggerPodName2)
	if err := client.CreatePod(loggerPod2, common.WithService(loggerPodName2)); err != nil {
		t.Fatalf("Failed to create logger service %q: %v", loggerPodName2, err)
	}

	// create subscription
	if err := client.CreateSubscription(
		subscriptionName,
		channelName,
		base.WithSubscriberForSubscription(loggerPodName2),
	); err != nil {
		t.Fatalf("Error creating subscription %q: %v", subscriptionName, err)
	}

	// wait for all test resources to be ready, so that we can start sending events
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// send fake CloudEvent to the broker
	eventToSend := &base.CloudEvent{
		Source:   eventSource1,
		Type:     eventType1,
		Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
		Encoding: base.CloudEventDefaultEncoding,
	}
	if err := client.SendFakeEventToBroker(senderName, brokerName, eventToSend); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
	}

	// check if trigger2's logging service receives both events
	eventBodies := []string{transformedEventBody, eventBody}
	if err := client.CheckLog(loggerPodName1, common.CheckerContainsAll([]string{transformedEventBody, eventBody})); err != nil {
		t.Fatalf("Strings %v not found in logs of logger pod %q: %v", eventBodies, loggerPodName1, err)
	}

	// check if subscription's logging service receives the transformed event
	if err := client.CheckLog(loggerPodName2, common.CheckerContains(transformedEventBody)); err != nil {
		t.Fatalf("Strings %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName2, err)
	}
}
