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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

	"github.com/knative/eventing/test"
	"github.com/knative/pkg/test/logging"
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

type TypeAndSource struct {
	Type, Source string
}

func name(brokerName, eventType, eventSource string) string {
	return fmt.Sprintf("%s-%s-%s", brokerName, eventType, eventSource)
}

func TestBrokerTrigger(t *testing.T) {
	logger := logging.GetContextLogger("TestBrokerTrigger")

	clients, cleaner := Setup(t, logger)
	defer TearDown(clients, cleaner, logger)

	// verify namespace and annotate to create default broker
	ns, cleanupNS := NamespaceExists(t, clients, logger, true)
	defer cleanupNS()

	defaultBroker := test.Broker(defaultBrokerName, ns)
	err := WaitForBrokerReady(clients, defaultBroker)
	if err != nil {
		t.Fatalf("Error waiting for default broker to become ready: %v", err)
	}

	// name -> selector
	eventLoggers := []map[string]map[string]string{
		{name(defaultBroker, any, any): {selectorKey: string(uuid.NewUUID())}},
		{name(defaultBroker, eventType1, any): {selectorKey: string(uuid.NewUUID())}},
		{name(defaultBroker, any, eventSource1): {selectorKey: string(uuid.NewUUID())}},
		{name(defaultBroker, eventType1, eventSource1): {selectorKey: string(uuid.NewUUID())}},
	}

	logger.Info("Creating Subscriber pods")

	for name, selector := range eventLoggers {
		subscriberPod := test.EventLoggerPod(name, ns, selector)
		if err := CreatePod(clients, subscriberPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create subscriber pod: %v", err)
		}
	}

	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event logger pod to become running: %v", err)
	}

	logger.Info("Subscriber pods running")

	logger.Info("Creating Subscriber services")

	for name, selector := range eventLoggers {
		subscriberSvc := test.Service(name, ns, selector)
		if err := WithServiceReady(clients, subscriberSvc, logger, cleaner); err != nil {
			t.Fatalf("Error waiting for subscriber service to become ready: %v", err)
		}
	}

	logger.Info("Subscriber services ready")

	logger.Info("Creating Triggers")

	for name, selector := range eventLoggers {
		strs := strings.Split(name, "-")
		_, brokerName, eventType, eventSource := strs[0], strs[1], strs[2], strs[3]
		trigger := test.Trigger(name, ns, eventType, eventSource, name)
		err := WithTriggerReady(clients, defaultTrigger, logger, cleaner)
		if err != nil {
			t.Fatalf("Error waiting for trigger to become ready: %v", err)
		}
	}

	logger.Info("Triggers ready")

	typesAndSources = []TypeAndSource{
		{eventType1, eventSource1},
		{eventType1, eventSource2},
		{eventType2, eventSource1},
		{eventType2, eventSource2},
	}

	logger.Infof("Creating event sender pods")
	// TODO should create a single pod that can send multiple events
	for event := range typesAndSources {
		body := fmt.Sprintf("Testing Broker-Trigger %s", uuid.NewUUID())
		cloudEvent := test.CloudEvent{
			Source:   event.Source,
			Type:     event.Type,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: encoding,
		}
		url := fmt.Sprintf("http://%s", defaultBroker.Status.Address.Hostname)
		senderPod := test.EventSenderPod(event.Source, ns, url, cloudEvent)
		logger.Infof("Sender pod: %#v", senderPod)
		if err := CreatePod(clients, senderPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create event sender pod: %v", err)
		}
	}

	// Verify that each event arrived to its own place.
	if err := WaitForLogContent(clients, logger, routeName, subscriberPod.Spec.Containers[0].Name, ns, body); err != nil {
		t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body, routeName, err)
	}

}
