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
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	defaultBrokerName = "default"
	selectorKey       = "end2end-test-broker-trigger"

	any          = v1alpha1.TriggerAnyFilter
	eventType1   = "type1"
	eventType2   = "type2"
	eventSource1 = "source1"
	eventSource2 = "source2"
)

// Helper function to create names for different objects (e.g., triggers, services, etc.)
func name(obj, brokerName, eventType, eventSource string) string {
	return fmt.Sprintf("%s-%s-%s-%s", obj, brokerName, eventType, eventSource)
}

// Helper object to easily create subscriber pods, services and triggers.
// We also use this to verify the expected events that should be received
// by the particular subscriber pods.
type DumperInfo struct {
	Namespace      string
	Broker         string
	EventType      string
	EventSource    string
	Selector       map[string]string
	ExpectedBodies []string
}

// Helper object to easily create event sender pods.
type SenderInfo struct {
	Namespace   string
	Url         string
	EventType   string
	EventSource string
}

func TestBrokerTrigger(t *testing.T) {
	logger := logging.GetContextLogger("TestBrokerTrigger")

	clients, cleaner := Setup(t, logger)
	defer TearDown(clients, cleaner, logger)

	// Verify namespace and annotate to create default broker.
	ns, cleanupNS := NamespaceExists(t, clients, logger, true)
	defer cleanupNS()

	// Wait for default broker ready.
	defaultBroker := test.Broker(defaultBrokerName, ns)
	err := WaitForBrokerReady(clients, defaultBroker)
	if err != nil {
		t.Fatalf("Error waiting for default broker to become ready: %v", err)
	}

	defaultBrokerUrl := fmt.Sprintf("http://%s", defaultBroker.Status.Address.Hostname)

	// Create sender helpers.
	senders := []SenderInfo{
		{ns, defaultBrokerUrl, eventType1, eventSource1},
		{ns, defaultBrokerUrl, eventType1, eventSource2},
		{ns, defaultBrokerUrl, eventType2, eventSource1},
		{ns, defaultBrokerUrl, eventType2, eventSource2},
	}

	// Create DumperInfo helpers.
	dumpers := []DumperInfo{
		{ns, defaultBrokerName, any, any, {selectorKey: string(uuid.NewUUID())}, make([]string, 0)},
		{ns, defaultBrokerName, eventType1, any, {selectorKey: string(uuid.NewUUID())}, make([]string, 0)},
		{ns, defaultBrokerName, any, eventSource1, {selectorKey: string(uuid.NewUUID())}, make([]string, 0)},
		{ns, defaultBrokerName, eventType1, eventSource1, {selectorKey: string(uuid.NewUUID())}, make([]string, 0)},
	}

	logger.Info("Creating Subscriber pods")

	// Save the references in this map for later use.
	subscriberPods := make(map[string]*corev1.Pod, 0)
	for _, dumper := range dumpers {
		subscriberPodName := name("dumper", dumper.Broker, dumper.EventType, dumper.EventSource)
		subscriberPod := test.EventLoggerPod(subscriberPodName, dumper.Namespace, dumper.Selector)
		if err := CreatePod(clients, subscriberPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create subscriber pod: %v", err)
		}
		subscriberPods[subscriberPodName] = subscriberPod
	}

	// Wait for all of them to be running.
	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event logger pod to become running: %v", err)
	}

	logger.Info("Subscriber pods running")

	logger.Info("Creating Subscriber services")

	for _, dumper := range dumpers {
		subscriberSvcName := name("svc", dumper.Broker, dumper.EventType, dumper.EventSource)
		subscriberSvc := test.Service(subscriberSvcName, dumper.Namespace, dumper.Selector)
	}

	logger.Info("Subscriber services created")

	logger.Info("Creating Triggers")

	for _, dumper := range dumpers {
		triggerName := name("trigger", dumper.Broker, dumper.EventType, dumper.EventSource)
		// subscriberName should be the same as the subscriberSvc from before.
		subscriberName := name("svc", dumper.Broker, dumper.EventType, dumper.EventSource)
		trigger := test.Trigger(triggerName, dumper.Namespace, dumper.EventType, dumper.EventSource, dumper.Broker, subscriberName)
		// Wait for the triggers to be ready
		err := WithTriggerReady(clients, trigger, logger, cleaner)
		if err != nil {
			t.Fatalf("Error waiting for trigger to become ready: %v", err)
		}
	}

	logger.Info("Triggers ready")

	logger.Info("Creating event sender pods")

	for _, sender := range senders {
		// Create cloud event.
		body := fmt.Sprintf("Testing Broker-Trigger %s", uuid.NewUUID())
		cloudEvent := test.CloudEvent{
			Source:   sender.EventSource,
			Type:     sender.EventType,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: test.CloudEventEncodingStructured,
		}
		// Create sender pod.
		senderPodName := fmt.Sprintf("sender-%s-%s", sender.EventType, sender.EventSource)
		senderPod := test.EventSenderPod(senderPodName, sender.Namespace, url, cloudEvent)
		logger.Infof("Sender pod: %#v", senderPod)
		if err := CreatePod(clients, senderPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create event sender pod: %v", err)
		}

		// Check on every dumper whether we should expect this event or not, and add its body if so.
		for _, dumper := range dumpers {
			if shouldExpectEvent(dumper, sender) {
				dumper.ExpectedBodies = append(dumper.ExpectedBodies, body)
			}
		}
	}

	logger.Info("Created event sender pods")

	// Wait for all of them to be running.
	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event sender pod to become running: %v", err)
	}

	logger.Info("Verifying events arrived to appropriate dumpers")

	for _, dumper := range dumpers {
		subscriberPodName := name("dumper", dumper.Broker, dumper.EventType, dumper.EventSource)
		subscriberPod := subscriberPods[subscriberPodName]
		if err := WaitForLogContents(clients, logger, routeName, subscriberPod.Spec.Containers[0].Name, dumper.Namespace, dumper.ExpectedBodies); err != nil {
			t.Fatalf("String(s) not found in logs of subscriber pod %q: %v", subscriberPodName, err)
		}
	}

	logger.Info("Successfully completed!")

}

func shouldExpectEvent(dumper *DumperInfo, sender *SenderInfo) bool {
	if dumper.Namespace != sender.Namespace {
		return false
	}
	if dumper.EventType != any && dumper.EventType != sender.EventType {
		return false
	}
	if dumper.EventSource != any && dumper != sender.EventSource {
		return false
	}
	return true
}
