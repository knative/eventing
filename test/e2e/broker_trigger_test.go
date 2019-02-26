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
	defaultBrokerName       = "default"
	waitForFilterPodRunning = 30 * time.Second
	selectorKey             = "end2end-test-broker-trigger"

	any          = v1alpha1.TriggerAnyFilter
	eventType1   = "type1"
	eventType2   = "type2"
	eventSource1 = "source1"
	eventSource2 = "source2"
)

// Helper struct to tie the type and sources of the events we expect to receive
// in subscribers with the selectors we use when creating their pods.
type eventReceiver struct {
	typeAndSource test.TypeAndSource
	selector      map[string]string
}

// This test annotates the testing namespace so that a default broker is created.
// It then binds many triggers with different filtering patterns to that default broker,
// and sends different events to the broker's address. Finally, it verifies that only
// the appropriate events are routed to the subscribers.
func TestDefaultBrokerWithManyTriggers(t *testing.T) {
	logger := logging.GetContextLogger("TestDefaultBrokerWithManyTriggers")

	clients, cleaner := Setup(t, logger)

	// Verify namespace exists.
	ns, cleanupNS := NamespaceExists(t, clients, logger)

	defer cleanupNS()
	defer TearDown(clients, cleaner, logger)

	logger.Infof("Labeling namespace %s", ns)

	// Label namespace so that it creates the default broker.
	err := LabelNamespace(clients, logger, map[string]string{"knative-eventing-injection": "enabled"})
	if err != nil {
		t.Fatalf("Error annotating namespace: %v", err)
	}

	logger.Infof("Namespace %s annotated", ns)

	// Wait for default broker ready.
	logger.Info("Waiting for default broker to be ready")
	defaultBroker := test.Broker(defaultBrokerName, ns)
	err = WaitForBrokerReady(clients, defaultBroker)
	if err != nil {
		t.Fatalf("Error waiting for default broker to become ready: %v", err)
	}

	defaultBrokerUrl := fmt.Sprintf("http://%s", defaultBroker.Status.Address.Hostname)

	logger.Infof("Default broker ready: %q", defaultBrokerUrl)

	// These are the event types and sources that triggers will listen to, as well as the selectors
	// to set  in the subscriber and services pods.
	eventsToReceive := []eventReceiver{
		{test.TypeAndSource{Type: any, Source: any}, newSelector()},
		{test.TypeAndSource{Type: eventType1, Source: any}, newSelector()},
		{test.TypeAndSource{Type: any, Source: eventSource1}, newSelector()},
		{test.TypeAndSource{Type: eventType1, Source: eventSource1}, newSelector()},
	}

	logger.Info("Creating Subscriber pods")

	// Save the pods references in this map for later use.
	subscriberPods := make(map[string]*corev1.Pod, len(eventsToReceive))
	for _, event := range eventsToReceive {
		subscriberPodName := name("dumper", event.typeAndSource.Type, event.typeAndSource.Source)
		subscriberPod := test.EventLoggerPod(subscriberPodName, ns, event.selector)
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
		subscriberSvcName := name("svc", event.typeAndSource.Type, event.typeAndSource.Source)
		service := test.Service(subscriberSvcName, ns, event.selector)
		if err := CreateService(clients, service, logger, cleaner); err != nil {
			t.Fatalf("Error creating subscriber service: %v", err)
		}
	}

	logger.Info("Subscriber services created")

	logger.Info("Creating Triggers")

	for _, event := range eventsToReceive {
		triggerName := name("trigger", event.typeAndSource.Type, event.typeAndSource.Source)
		// subscriberName should be the same as the subscriberSvc from before.
		subscriberName := name("svc", event.typeAndSource.Type, event.typeAndSource.Source)
		trigger := test.NewTriggerBuilder(triggerName, ns).
			EventType(event.typeAndSource.Type).
			EventSource(event.typeAndSource.Source).
			// Don't need to set the broker as we use the default one
			// but wanted to be more explicit.
			Broker(defaultBrokerName).
			SubscriberSvc(subscriberName).
			Build()
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

	// We notice some crashLoopBacks in the filter and ingress pod creation.
	// We then delay the creation of the sender pods in order not to miss events.
	// TODO improve this
	logger.Info("Waiting for filter and ingress pods to become running")
	time.Sleep(waitForFilterPodRunning)

	logger.Info("Creating event sender pods")

	// Map to save the expected events per dumper so that we can verify the delivery.
	expectedEvents := make(map[string][]string)
	// Map to save the unexpected events per dumper so that we can verify that they weren't delivered.
	unexpectedEvents := make(map[string][]string)
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
		// to the expectedEvents/unexpectedEvents maps.
		for _, eventToReceive := range eventsToReceive {
			subscriberPodName := name("dumper", eventToReceive.typeAndSource.Type, eventToReceive.typeAndSource.Source)
			if shouldExpectEvent(&eventToSend, &eventToReceive, logger) {
				expectedEvents[subscriberPodName] = append(expectedEvents[subscriberPodName], body)
			} else {
				unexpectedEvents[subscriberPodName] = append(unexpectedEvents[subscriberPodName], body)
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
		subscriberPodName := name("dumper", event.typeAndSource.Type, event.typeAndSource.Source)
		subscriberPod := subscriberPods[subscriberPodName]
		logger.Infof("Dumper %q expecting %q", subscriberPodName, strings.Join(expectedEvents[subscriberPodName], ","))
		if err := WaitForLogContents(clients, logger, subscriberPodName, subscriberPod.Spec.Containers[0].Name, ns, expectedEvents[subscriberPodName]); err != nil {
			t.Fatalf("Event(s) not found in logs of subscriber pod %q: %v", subscriberPodName, err)
		}
		// At this point all the events should have been received in the pod.
		// We check whether we find unexpected events. If so, then we fail.
		found, err := FindAnyLogContents(clients, logger, subscriberPodName, subscriberPod.Spec.Containers[0].Name, ns, unexpectedEvents[subscriberPodName])
		if err != nil {
			t.Fatalf("Failed querying to find log contents in pod %q: %v", subscriberPodName, err)
		}
		if found {
			t.Fatalf("Unexpected event(s) found in logs of subscriber pod %q", subscriberPodName)
		}
	}
}

// Helper function to create names for different objects (e.g., triggers, services, etc.).
func name(obj, eventType, eventSource string) string {
	// Pod names need to be lowercase. We might have an eventType as Any, that is why we lowercase them.
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", obj, eventType, eventSource))
}

// Returns a new selector with a random uuid.
func newSelector() map[string]string {
	return map[string]string{selectorKey: string(uuid.NewUUID())}
}

// Checks whether we should expect to receive 'eventToSend' in 'eventReceiver' based on its type and source pattern.
func shouldExpectEvent(eventToSend *test.TypeAndSource, receiver *eventReceiver, logger *logging.BaseLogger) bool {
	if receiver.typeAndSource.Type != any && receiver.typeAndSource.Type != eventToSend.Type {
		logger.Debugf("Event types mismatch, receive %s, send %s", receiver.typeAndSource.Type, eventToSend.Type)
		return false
	}
	if receiver.typeAndSource.Source != any && receiver.typeAndSource.Source != eventToSend.Source {
		logger.Debugf("Event sources mismatch, receive %s, send %s", receiver.typeAndSource.Source, eventToSend.Source)
		return false
	}
	return true
}
