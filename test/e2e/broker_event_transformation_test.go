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
	RunTests(t, common.FeatureBasic, testEventTransformationForTrigger)
}

func testEventTransformationForTrigger(t *testing.T, provisioner string) {
	const (
		senderName    = "e2e-eventtransformation-sender"
		brokerName    = "e2e-eventtransformation-broker"
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
		eventBody    = "e2e-eventtransformation-body"

		triggerName1 = "trigger1"
		triggerName2 = "trigger2"

		transformationPodName = "trans-pod"
		loggerPodName         = "logger-pod"
	)

	client := Setup(t, provisioner, true, t.Logf)
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
	eventAfterTransformation := &common.CloudEvent{
		Source:   eventSource2,
		Type:     eventType2,
		Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
		Encoding: common.CloudEventDefaultEncoding,
	}

	// create the transformation service
	if err := client.CreateTransformationService(transformationPodName, eventAfterTransformation); err != nil {
		t.Fatalf("Failed to create transformation service %q: %v", transformationPodName, err)
	}

	// create trigger1 for event transformation
	triggerFilter1 := base.TriggerFilter(eventSource1, eventType1)
	if err := client.CreateTrigger(triggerName1, brokerName, triggerFilter1, transformationPodName); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName1, err)
	}

	// create logger service to receive events for validation
	if err := client.CreateLoggerService(loggerPodName); err != nil {
		t.Fatalf("Failed to create logger service %q: %v", loggerPodName, err)
	}

	// create trigger2 for event receiving
	triggerFilter2 := base.TriggerFilter(eventSource2, eventType2)
	if err := client.CreateTrigger(triggerName2, brokerName, triggerFilter2, loggerPodName); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName2, err)
	}

	// wait for all test resources to be ready, so that we can start sending events
	client.WaitForAllTestResourcesReady()

	// send fake CloudEvent to the broker
	eventToSend := &common.CloudEvent{
		Source:   eventSource1,
		Type:     eventType1,
		Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
		Encoding: common.CloudEventDefaultEncoding,
	}
	if err := client.SendFakeEventToBroker(senderName, brokerName, eventToSend); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
	}

	// check if the logging service receives the correct event
	if err := client.CheckLogContent(loggerPodName, transformedEventBody); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName, err)
	}
}
