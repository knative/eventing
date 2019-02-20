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
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	defaultBrokerName = "default"

	eventAny     = v1alpha1.TriggerAnyFilter
	eventType1   = "test.type1"
	eventType2   = "test.type2"
	eventSource1 = "test.source1"
	eventSource2 = "test.source2"

	defaultAnyDumper          = "default-any-dumper"
	defaultType1Dumper        = "default-type1-dumper"
	defaultSource1Dumper      = "default-source1-dumper"
	defaultType1Source1Dumper = "default-type1-source1-dumper"
)

type TypeAndSource struct {
	Type, Source string
}

func triggerName(broker, eventType string) string {
	fmt.Sprintf("%s-dump-%s", broker, eventType)
}

func TestBrokerTrigger(t *testing.T) {
	logger := logging.GetContextLogger("TestBrokerTrigger")

	clients, cleaner := Setup(t, logger)
	defer TearDown(clients, cleaner, logger)

	// verify namespace
	ns, cleanupNS := NamespaceExists(t, clients, logger)
	defer cleanupNS()

	// TODO label namespace to get default Broker

	logger.Info("Creating Subscriber pods")

	eventLoggerSelectors := []map[string]string{{"svcName": defaultAnyDumper}, {"svcName": defaultType1Dumper}, {"svcName": defaultSource1Dumper}, {"svcName": defaultType1Source1Dumper}}

	for eventLoggerSelector := range eventLoggerSelectors {
		subscriberPod := test.EventLoggerPod(eventLoggerSelector["svcName"], ns, eventLoggerSelector)

		if err := CreatePod(clients, subscriberPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create event logger pod: %v", err)
		}
	}

	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for event logger pod to become running: %v", err)
	}

	logger.Info("Subscriber pods running")

	for eventLoggerSelector := range eventLoggerSelectors {
		subscriberSvc := test.Service(eventLoggerSelector["svcName"], ns, eventLoggerSelector)
		if err := CreateService(clients, subscriberSvc, logger, cleaner); err != nil {
			t.Fatalf("Failed to create event logger service: %v", err)
		}
	}

	logger.Info("Creating Triggers")

	defaultTriggers := []*v1alpha1.Trigger{
		test.Trigger("default-dump-any", ns, eventAny, eventAny, defaultBrokerName, defaultAnyDumper),
		test.Trigger("default-dump-type1", ns, eventType1, eventAny, defaultBrokerName, defaultType1Dumper),
		test.Trigger("default-dump-source1", ns, eventAny, eventSource1, defaultBrokerName, defaultSource1Dumper),
		test.Trigger("default-dump-type1-source1", ns, eventType1, eventSource1, defaultBrokerName, defaultType1Source1Dumper),
	}
	for defaultTrigger := range defaultTriggers {
		err := WithTriggerReady(clients, defaultTrigger, logger, cleaner)
		if err != nil {
			t.Fatalf("Error waiting for default trigger to become ready: %v", err)
		}
	}

	logger.Info("Triggers ready")

	logger.Infof("Creating event sender pods")

	events = []TypeAndSource{
		{eventType1, eventSource1},
		{eventType1, eventSource2},
		{eventType2, eventSource1},
		{eventType2, eventSource2},
	}

	for event := range events {
		body := fmt.Sprintf("Testing Broker-Trigger %s", uuid.NewUUID())
		cloudEvent := test.CloudEvent{
			Source:   event.Source,
			Type:     event.Type,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: encoding,
		}
		url := fmt.Sprintf("http://%s", channel.Status.Address.Hostname)
		senderPod := test.EventSenderPod(event.Source, ns, url, cloudEvent)
		logger.Infof("Sender pod: %#v", senderPod)
		if err := CreatePod(clients, senderPod, logger, cleaner); err != nil {
			t.Fatalf("Failed to create event sender pod: %v", err)
		}
	}

	// Verify that each arrived to its own place.
	//if err := WaitForLogContent(clients, logger, routeName, subscriberPod.Spec.Containers[0].Name, ns, body); err != nil {
	//	t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body, routeName, err)
	//}

}
