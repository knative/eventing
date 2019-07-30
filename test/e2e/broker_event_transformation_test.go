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
	"github.com/knative/eventing/test/base/resources"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestEventTransformationForTrigger tests the following scenario:

                         5                 4
                   ------------- ----------------------
                   |           | |                    |
             1     v	 2     | v        3           |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   | 6                   7
                   |-------> Trigger2 -------> Service(Logger)

Note: the number denotes the sequence of the event that flows in this test case.
*/
func TestEventTransformationForTrigger(t *testing.T) {
	runTests(t, channels, common.FeatureBasic, testEventTransformationForTrigger)
}

func testEventTransformationForTrigger(t *testing.T, channel string) {
	const (
		senderName = "e2e-eventtransformation-sender"
		brokerName = "e2e-eventtransformation-broker"

		any          = v1alpha1.TriggerAnyFilter
		eventType1   = "type1"
		eventType2   = "type2"
		eventSource1 = "source1"
		eventSource2 = "source2"
		eventBody    = "e2e-eventtransformation-body"

		triggerName1 = "trigger1"
		triggerName2 = "trigger2"

		transformationPodName = "trans-pod"
		loggerPodName         = "logger-pod"
	)

	client := setup(t, true)
	defer tearDown(client)
	channelTypeMeta := getChannelTypeMeta(channel)

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

	// create the transformation service
	transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
	client.CreatePodOrFail(transformationPod, common.WithService(transformationPodName))

	// create trigger1 for event transformation
	client.CreateTriggerOrFail(
		triggerName1,
		resources.WithBroker(brokerName),
		resources.WithTriggerFilter(eventSource1, eventType1),
		resources.WithSubscriberRefForTrigger(transformationPodName),
	)

	// create logger pod and service
	loggerPod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

	// create trigger2 for event receiving
	client.CreateTriggerOrFail(
		triggerName2,
		resources.WithBroker(brokerName),
		resources.WithTriggerFilter(eventSource2, eventType2),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// send fake CloudEvent to the broker
	eventToSend := &resources.CloudEvent{
		Source:   eventSource1,
		Type:     eventType1,
		Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
		Encoding: resources.CloudEventDefaultEncoding,
	}
	if err := client.SendFakeEventToAddressable(senderName, brokerName, common.BrokerTypeMeta, eventToSend); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
	}

	// check if the logging service receives the correct event
	if err := client.CheckLog(loggerPodName, common.CheckerContains(transformedEventBody)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName, err)
	}
}
