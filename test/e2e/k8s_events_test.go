// +build e2e

/*
Copyright 2018 The Knative Authors
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
	"testing"
	"time"

	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
)

const (
	serviceAccount = "e2e-feed"
	eventSource    = "k8sevents"
	eventType      = "dev.knative.k8s.event"
	routeName      = "e2e-k8s-events-function"
	flowName       = "e2e-k8s-events-example"
)

func TestKubernetesEvents(t *testing.T) {

	logger := logging.GetContextLogger("TestKubernetesEvents")

	clients, cleaner := Setup(t, logger)

	test.CleanupOnInterrupt(func() { TearDown(clients, cleaner, logger) }, logger)
	defer TearDown(clients, cleaner, logger)

	logger.Infof("Creating ServiceAccount and Binding")

	err := CreateServiceAccountAndBinding(clients, serviceAccount, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount or Binding: %v", err)
	}

	logger.Infof("Creating ClusterBus")

	// The dispatcher used by the ClusterBus to dispatch messages between channels and subscriptions
	dispatcherImagePath := ImagePath("github.com/knative/eventing/pkg/buses/stub/dispatcher")
	stub := test.ClusterBus("stub", defaultNamespaceName, dispatcherImagePath)
	err = CreateClusterBus(clients, stub, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create ClusterBus: %v", err)
	}

	logger.Infof("Creating EventSource")

	// The event source wrapper for Kubernetes events
	eventSourceImagePath := ImagePath("github.com/knative/eventing/pkg/sources/k8sevents")
	// The actual event source which receives events and posts them to the given Route
	receiverAdapterImagePath := ImagePath("github.com/knative/eventing/pkg/sources/k8sevents/receive_adapter")
	eSource := test.EventSource(eventSource, defaultNamespaceName, eventSourceImagePath, receiverAdapterImagePath)
	err = CreateEventSource(clients, eSource, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create EventSource: %v", err)
	}

	logger.Infof("Creating EventType")

	eType := test.EventType(eventType, defaultNamespaceName, eventSource)
	err = CreateEventType(clients, eType, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create EventType: %v", err)
	}

	logger.Infof("Creating Route and Config")

	// The receiver of events which is accessible through Route
	configImagePath := ImagePath("github.com/knative/eventing/test/e2e/k8sevents")
	err = WithRouteReady(clients, logger, cleaner, routeName, configImagePath)
	if err != nil {
		t.Fatalf("The Route was not marked as Ready to serve traffic: %v", err)
	}

	logger.Infof("Creating Flow")

	flow := test.Flow(flowName, defaultNamespaceName, serviceAccount, eventType, eventSource, routeName, testNamespace)
	err = WithFlowReady(clients, flow, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Flow: %v", err)
	}

	//Work around for: https://github.com/knative/eventing/issues/125
	//and the fact that even after pods are up, due to Istio slowdown, there's
	//about 5-6 seconds that traffic won't be passed through.
	WaitForAllPodsRunning(clients, logger, pkgTest.Flags.Namespace)
	time.Sleep(10 * time.Second)

	logger.Infof("Creating Pod")

	err = CreatePod(clients, test.NGinxPod(testNamespace), logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}

	WaitForAllPodsRunning(clients, logger, testNamespace)

	err = WaitForLogContent(clients, logger, routeName, "user-container", "Created container")
	if err != nil {
		t.Fatalf("Events for container created not received: %v", err)
	}
	err = WaitForLogContent(clients, logger, routeName, "user-container", "Started container")
	if err != nil {
		t.Fatalf("Events for container started not received: %v", err)
	}
}
