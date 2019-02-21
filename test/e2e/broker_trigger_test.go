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
	"strings"
	"testing"
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

	"github.com/knative/eventing/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	defaultBrokerName            = "default"
	waitForDefaultBrokerCreation = 3 * time.Second
	selectorKey                  = "end2end-test-broker-trigger"

	any          = v1alpha1.TriggerAnyFilter
	eventType1   = "type1"
	eventType2   = "type2"
	eventSource1 = "source1"
	eventSource2 = "source2"
)

// Helper function to create names for different objects (e.g., triggers, services, etc.)
func name(obj, eventType, eventSource string) string {
	// pod names need to be lowercase. We might have an eventType as Any, that is why we lowercase them.
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", obj, eventType, eventSource))
}

func TestDefaultBrokerWithManyTriggers(t *testing.T) {
	logger := logging.GetContextLogger("TestDefaultBrokerWithManyTriggers")

	clients, cleaner := Setup(t, logger)
	defer TearDown(clients, cleaner, logger)

	// Verify namespace exists.
	ns, cleanupNS := NamespaceExists(t, clients, logger)
	defer cleanupNS()

	logger.Infof("Annotating namespace %s", ns)

	// Annotate namespace so that it creates the default broker.
	err := AnnotateNamespace(clients, logger, map[string]string{"eventing.knative.dev/inject": "true"})
	if err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}

	logger.Infof("Namespace %s annotated", ns)

	// As we are not creating the default broker,
	// we wait for a few seconds for the broker to get created.
	// Otherwise, if we try to wait for its Ready status and the namespace controller
	// didn't actually create it yet, the test will fail.
	// TODO improve this
	logger.Info("Waiting for default broker creation")
	time.Sleep(waitForDefaultBrokerCreation)

	// Wait for default broker ready.
	logger.Info("Waiting for default broker to be ready")
	defaultBroker := test.Broker(defaultBrokerName, ns)
	err = WaitForBrokerReady(clients, defaultBroker)
	if err != nil {
		t.Fatalf("Error waiting for default broker to become ready: %v", err)
	}

	defaultBrokerUrl := fmt.Sprintf("http://%s", defaultBroker.Status.Address.Hostname)

	logger.Infof("Default broker ready: %q", defaultBrokerUrl)

	// These are the event types and sources that triggers will listen to.
	eventsToReceive := []test.TypeAndSource{
		{any, any},
		{eventType1, any},
		{any, eventSource1},
		{eventType1, eventSource1},
	}

	selector := map[string]string{selectorKey: string(uuid.NewUUID())}

	logger.Info("Creating Subscriber pods")

	// Save the pods references in this map for later use.
	subscriberPods := make(map[string]*corev1.Pod, 0)
	for _, event := range eventsToReceive {
		subscriberPodName := name("dumper", event.Type, event.Source)
		subscriberPod := test.EventLoggerPod(subscriberPodName, ns, selector)
		if err := CreatePod(clients, subscriberPod, logger, cleaner); err != nil {
			t.Fatalf("Error creating subscriber pod: %v", err)
		}
		subscriberPods[subscriberPodName] = subscriberPod
	}

	logger.Info("Subscriber pods created")

	logger.Info("Waiting for subscriber pods to become running")

	// Wait for all of the pods in the namespace to become running.
	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event logger pod to become running: %v", err)
	}

	logger.Info("Subscriber pods running")

	logger.Info("Creating Subscriber services")

	for _, event := range eventsToReceive {
		subscriberSvcName := name("svc", event.Type, event.Source)
		service := test.Service(subscriberSvcName, ns, selector)
		if err := CreateService(clients, service, logger, cleaner); err != nil {
			t.Fatalf("Error creating subscriber service: %v", err)
		}
	}

	logger.Info("Subscriber services created")

	logger.Info("Creating Triggers")

	for _, event := range eventsToReceive {
		triggerName := name("trigger", event.Type, event.Source)
		// subscriberName should be the same as the subscriberSvc from before.
		subscriberName := name("svc", event.Type, event.Source)
		trigger := test.Trigger(triggerName, ns, event.Type, event.Source, defaultBrokerName, subscriberName)
		err := CreateTrigger(clients, trigger, logger, cleaner)
		if err != nil {
			t.Fatalf("Error creating trigger: %v", err)
		}
	}

	logger.Info("Triggers created")

	logger.Info("Waiting for triggers to become ready")

	// Wait for all of the triggers in the namespace to be ready.
	if err := WaitForAllTriggersReady(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for triggers to become ready: %v", err)
	}

	logger.Info("Triggers ready")

	// These are the event types and sources that will be send.
	eventsToSend := []test.TypeAndSource{
		{eventType1, eventSource1},
		{eventType1, eventSource2},
		{eventType2, eventSource1},
		{eventType2, eventSource2},
	}

	logger.Info("Creating event sender pods")

	// Map to save the expected events per dumper so that we can verify the delivery.
	expectedEvents := make(map[string][]string)
	for _, eventToSend := range eventsToSend {
		// Create cloud event.
		// Using event type and source as part of the body for easier debugging.
		body := fmt.Sprintf("Body-%s-%s", eventToSend.Type, eventToSend.Source)
		cloudEvent := test.CloudEvent{
			Source: eventToSend.Source,
			Type:   eventToSend.Type,
			Data:   fmt.Sprintf(`{"msg":%q}`, body),
		}
		// Create sender pod.
		senderPodName := name("sender", eventToSend.Type, eventToSend.Source)
		senderPod := test.EventSenderPod(senderPodName, ns, defaultBrokerUrl, cloudEvent)
		if err := CreatePod(clients, senderPod, logger, cleaner); err != nil {
			t.Fatalf("Error creating event sender pod: %v", err)
		}

		// Check on every dumper whether we should expect this event or not, and add its body
		// to the expectedEvents map if so.
		for _, eventToReceive := range eventsToReceive {
			if shouldExpectEvent(&eventToSend, &eventToReceive, logger) {
				subscriberPodName := name("dumper", eventToReceive.Type, eventToReceive.Source)
				expectedEvents[subscriberPodName] = append(expectedEvents[subscriberPodName], body)
			}
		}
	}

	logger.Info("Event sender pods created")

	logger.Info("Waiting for event sender pods to be running")

	// Wait for all of them to be running.
	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event sender pod to become running: %v", err)
	}

	logger.Info("Event sender pods running")

	logger.Info("Verifying events delivered to appropriate dumpers")

	for _, event := range eventsToReceive {
		subscriberPodName := name("dumper", event.Type, event.Source)
		subscriberPod := subscriberPods[subscriberPodName]
		logger.Infof("Dumper %q expecting %q", subscriberPodName, strings.Join(expectedEvents[subscriberPodName], ","))
		if err := WaitForLogContents(clients, logger, subscriberPodName, subscriberPod.Spec.Containers[0].Name, ns, expectedEvents[subscriberPodName]); err != nil {
			t.Fatalf("String(s) not found in logs of subscriber pod %q: %v", subscriberPodName, err)
		}
	}

	logger.Info("Verification successful!")

}

func shouldExpectEvent(eventToSend *test.TypeAndSource, eventToReceive *test.TypeAndSource, logger *logging.BaseLogger) bool {
	if eventToReceive.Type != any && eventToReceive.Type != eventToSend.Type {
		logger.Debugf("Event types mismatch, receive %s, send %s", eventToReceive.Type, eventToSend.Type)
		return false
	}
	if eventToReceive.Source != any && eventToReceive.Source != eventToSend.Source {
		logger.Debugf("Event sources mismatch, receive %s, send %s", eventToReceive.Source, eventToSend.Source)
		return false
	}
	return true
}
