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
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"

	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"k8s.io/apimachinery/pkg/util/uuid"
)

// Helper to setup test data and expectations.
type testFixture struct {
	ingressPolicy       *v1alpha1.IngressPolicySpec
	preRegisterEvent    bool
	wantEventDelivered  bool
	wantEventRegistered bool
}

func TestRegistryBrokerAllowAnyAccept(t *testing.T) {
	//fixture := &testFixture{
	//	ingressPolicy: &v1alpha1.IngressPolicySpec{
	//		AllowAny: true,
	//	},
	//	wantEventDelivered: true,
	//}
	//Registry(t, fixture)
}

func TestRegistryBrokerAllowRegisteredAccept(t *testing.T) {
	//fixture := &testFixture{
	//	ingressPolicy: &v1alpha1.IngressPolicySpec{
	//		AllowAny: false,
	//	},
	//	preRegisterEvent:    true,
	//	wantEventDelivered:  true,
	//	wantEventRegistered: true,
	//}
	//Registry(t, fixture)
}

func TestRegistryBrokerAllowRegisteredNotAccept(t *testing.T) {
	//fixture := &testFixture{
	//	ingressPolicy: &v1alpha1.IngressPolicySpec{
	//		AllowAny: false,
	//	},
	//}
	//Registry(t, fixture)
}

func TestRegistryBrokerAutoAddAccept(t *testing.T) {
	//fixture := &testFixture{
	//	ingressPolicy: &v1alpha1.IngressPolicySpec{
	//		AutoAdd: true,
	//	},
	//	wantEventDelivered:  true,
	//	wantEventRegistered: true,
	//}
	//Registry(t, fixture)
}

func Registry(t *testing.T, fixture *testFixture) {
	clients, cleaner := Setup(t, t.Logf)

	ns, cleanupNS := CreateNamespaceIfNeeded(t, clients, t.Logf)

	defer cleanupNS()
	defer TearDown(clients, cleaner, t.Logf)

	// Define the constants here to avoid conflicts with other e2e tests.
	const (
		brokerName                   = "test-broker"
		triggerName                  = "test-trigger"
		eventTypeName                = "test-eventtype"
		subscriberName               = "test-dumper"
		waitTimeForBrokerPodsRunning = 30 * time.Second
	)

	t.Logf("Labeling Namespace %s", ns)
	err := LabelNamespace(clients, t.Logf, map[string]string{"knative-eventing-injection": "enabled"})
	if err != nil {
		t.Fatalf("Error labeling Namespace: %v", err)
	}
	t.Logf("Namespace %s labeled", ns)

	broker := test.NewBrokerBuilder(brokerName, ns).
		IngressPolicy(fixture.ingressPolicy).
		Build()
	t.Logf("Creating and waiting for Broker %s ready", broker.Name)
	err = WithBrokerReady(clients, broker, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Error creating and waiting for Broker ready: %v", err)
	}
	brokerUrl := fmt.Sprintf("http://%s", broker.Status.Address.Hostname)
	t.Logf("Broker created and ready: %q", brokerUrl)

	t.Logf("Creating Subscriber Pod and Service")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	subscriberPod := test.EventLoggerPod(subscriberName, ns, selector)
	subscriberSvc := test.Service(subscriberName, ns, selector)
	subscriberPod, err = CreatePodAndServiceReady(clients, subscriberPod, subscriberSvc, ns, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Subscriber Pod and Service, and get them ready: %v", err)
	}
	t.Logf("Subscriber Pod and Service created and ready")

	trigger := test.NewTriggerBuilder(triggerName, ns).
		Broker(broker.Name).
		SubscriberSvc(subscriberSvc.Name).
		Build()
	t.Logf("Creating and waiting for Trigger %s ready", trigger.Name)
	err = WithTriggerReady(clients, trigger, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Error creating and waiting for Trigger ready: %v", err)
	}
	t.Logf("Trigger created and ready")

	eventType := test.NewEventTypeBuilder(eventTypeName, ns).
		Broker(brokerName).
		// use the default values for source, type and schema.
		Build()
	if fixture.preRegisterEvent {
		t.Logf("Creating and waiting for EventType %s ready", eventType.Name)
		err = WithEventTypeReady(clients, eventType, t.Logf, cleaner)
		if err != nil {
			t.Fatalf("Error creating and waiting for EventType ready: %v", err)
		}
		t.Logf("EventType created and ready")
	}

	// We notice some crashLoopBacks in the Broker's filter and ingress pod creation.
	// We then delay the creation of the sender pod in order not to miss the event.
	t.Logf("Waiting for Broker's filter and ingress POD to become running")
	time.Sleep(waitTimeForBrokerPodsRunning)

	body := fmt.Sprintf("Registry %s", uuid.NewUUID())
	cloudEvent := &test.CloudEvent{
		Source: test.CloudEventDefaultSource,
		Type:   test.CloudEventDefaultType,
		Data:   fmt.Sprintf(`{"msg":%q}`, body),
	}
	t.Logf("Creating Sender Pod")
	senderPod := test.EventSenderPod("sender", ns, brokerUrl, cloudEvent)
	if err := CreatePod(clients, senderPod, t.Logf, cleaner); err != nil {
		t.Fatalf("Error creating event sender Pod: %v", err)
	}
	t.Logf("Sender Pod created. Waiting for it to be running")
	if err := pkgTest.WaitForAllPodsRunning(clients.Kube, ns); err != nil {
		t.Fatalf("Error waiting for Sender Pod to become running: %v", err)
	}
	t.Logf("Sender Pod running")

	if fixture.wantEventDelivered {
		t.Logf("Verifying Event delivered")
		if err := WaitForLogContents(clients, t.Logf, subscriberName, subscriberPod.Spec.Containers[0].Name, ns, []string{body}); err != nil {
			t.Fatalf("Event not found in logs of Subscriber Pod %q: %v", subscriberName, err)
		}
	} else {
		t.Logf("Verifying Event not delivered")
		found, err := FindAnyLogContents(clients, t.Logf, subscriberName, subscriberPod.Spec.Containers[0].Name, ns, []string{body})
		if err != nil {
			t.Fatalf("Failed querying to find log contents in Subscriber Pod %q: %v", subscriberName, err)
		}
		if found {
			t.Fatalf("Unexpected event found in logs of Subscriber Pod %q", subscriberName)
		}
	}

	// As the EventType might have been auto-generated, we cannot query for the particular EventType
	// with the name given by the constant eventTypeName. We just query for all of them, and see if all are ready.
	err = WaitForAllEventTypesReady(clients, t.Logf, ns)
	if fixture.wantEventRegistered {
		t.Logf("Verifying Event registered")
		if err != nil {
			t.Fatalf("EventType %q not ready: %v", eventType.Name, err)
		}
	} else {
		t.Logf("Verifying Event not registered")
		if err == nil {
			t.Fatalf("EventType %q registered and ready, but expected not to be", eventType.Name)
		}
	}
}
