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
	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
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
	const (
		brokerName = "e2e-eventtransformation-broker"
		saName     = "eventing-broker-filter"
		// This ClusterRole is installed in Knative Eventing setup, see https://github.com/knative/eventing/tree/master/docs/broker#manual-setup.
		crName = "eventing-broker-filter"

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

	clients, ns, provisioner, cleaner := Setup(t, true, t.Logf)
	defer TearDown(clients, ns, cleaner, t.Logf)

	// creates ServiceAccount and ClusterRoleBinding with default cluster-admin role
	err := CreateServiceAccountAndBinding(clients, saName, crName, ns, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create the ServiceAccount and ServiceAccountRoleBinding: %v", err)
	}

	// create a new broker
	broker := test.Broker(brokerName, ns, test.ClusterChannelProvisioner(provisioner))
	t.Logf("provisioner name is: %s", broker.Spec.ChannelTemplate.Provisioner.Name)
	err = WithBrokerReady(clients, broker, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Error waiting for the broker to become ready: %v, %v", err, broker)
	}
	brokerUrl := fmt.Sprintf("http://%s", broker.Status.Address.Hostname)
	t.Logf("The broker is ready with url: %q", brokerUrl)

	// create an event we want to send
	eventToSend := &test.CloudEvent{
		Source:   eventSource1,
		Type:     eventType1,
		Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
		Encoding: test.CloudEventDefaultEncoding,
	}

	// create the event we want to transform to
	transformedEventBody := fmt.Sprintf("%s %s", eventBody, string(uuid.NewUUID()))
	eventAfterTransformation := &test.CloudEvent{
		Source:   eventSource2,
		Type:     eventType2,
		Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
		Encoding: test.CloudEventDefaultEncoding,
	}

	// create the transformation pod and service, and get them ready
	transformationPodSelector := map[string]string{"e2etest": string(uuid.NewUUID())}
	transformationPod := test.EventTransformationPod(transformationPodName, ns, transformationPodSelector, eventAfterTransformation)
	transformationSvc := test.Service(transformationPodName, ns, transformationPodSelector)
	transformationPod, err = CreatePodAndServiceReady(clients, transformationPod, transformationSvc, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create transformation pod and service, and get them ready: %v", err)
	}

	trigger1 := test.NewTriggerBuilder(triggerName1, ns).
		EventType(eventType1).
		EventSource(eventSource1).
		Broker(brokerName).
		SubscriberSvc(transformationPodName).
		Build()
	err = CreateTrigger(clients, trigger1, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Error creating trigger1: %v", err)
	}

	// create logger pod and service, and get them ready
	loggerPodSelector := map[string]string{"e2etest": string(uuid.NewUUID())}
	loggerPod := test.EventLoggerPod(loggerPodName, ns, loggerPodSelector)
	loggerSvc := test.Service(loggerPodName, ns, loggerPodSelector)
	loggerPod, err = CreatePodAndServiceReady(clients, loggerPod, loggerSvc, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create logger pod and service, and get them ready: %v", err)
	}

	trigger2 := test.NewTriggerBuilder(triggerName2, ns).
		EventType(eventType2).
		EventSource(eventSource2).
		Broker(brokerName).
		SubscriberSvc(loggerPodName).
		Build()
	err = CreateTrigger(clients, trigger2, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Error creating trigger2: %v", err)
	}

	// Wait for all of the triggers in the namespace to be ready.
	if err := WaitForAllTriggersReady(clients, ns, t.Logf); err != nil {
		t.Fatalf("Error waiting for triggers to become ready: %v", err)
	}

	// send fake CloudEvent to the broker
	if err := SendFakeEventToBroker(clients, eventToSend, broker, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", broker.Name)
	}

	if err := pkgTest.WaitForLogContent(clients.Kube, loggerPodName, loggerPod.Spec.Containers[0].Name, ns, transformedEventBody); err != nil {
		logPodLogsForDebugging(clients, transformationPodName, transformationPod.Spec.Containers[0].Name, ns, t.Logf)
		logPodLogsForDebugging(clients, loggerPodName, loggerPod.Spec.Containers[0].Name, ns, t.Logf)
		logPodLogsForDebugging(clients, eventSource1, "sendevent", ns, t.Logf)
		t.Fatalf("String %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName, err)
	}
}
