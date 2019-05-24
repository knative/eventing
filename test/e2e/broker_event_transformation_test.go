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

	// create the transformation service
	transformationPod := base.EventTransformationPod(transformationPodName, eventAfterTransformation)
	if err := client.CreatePod(transformationPod, common.WithService(transformationPodName)); err != nil {
		t.Fatalf("Failed to create transformation service %q: %v", transformationPodName, err)
	}

	// create trigger1 for event transformation
	if err := client.CreateTrigger(
		triggerName1,
		base.WithBroker(brokerName),
		base.WithTriggerFilter(eventSource1, eventType1),
		base.WithSubscriberForTrigger(transformationPodName),
	); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName1, err)
	}

	// create logger pod and service
	loggerPod := base.EventLoggerPod(loggerPodName)
	if err := client.CreatePod(loggerPod, common.WithService(loggerPodName)); err != nil {
		t.Fatalf("Failed to create logger service %q: %v", loggerPodName, err)
	}

	// create trigger2 for event receiving
	if err := client.CreateTrigger(
		triggerName2,
		base.WithBroker(brokerName),
		base.WithTriggerFilter(eventSource2, eventType2),
		base.WithSubscriberForTrigger(loggerPodName),
	); err != nil {
		t.Fatalf("Error creating trigger %q: %v", triggerName2, err)
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

	// check if the logging service receives the correct event
	if err := client.CheckLog(loggerPodName, common.CheckerContains(transformedEventBody)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName, err)
	}
}
