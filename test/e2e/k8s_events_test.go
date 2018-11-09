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
	serviceAccount   = "e2e-receive-adapter"
	eventSource      = "k8sevents"
	routeName        = "e2e-k8s-events-function"
	channelName      = "e2e-k8s-events-channel"
	provisionerName  = "in-memory-channel"
	subscriptionName = "e2e-k8s-events-subscription"
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

	logger.Infof("Creating Channel")
	channel := test.Channel(channelName, defaultNamespaceName, test.ClusterChannelProvisioner(provisionerName))
	err = CreateChannel(clients, channel, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Channel: %v", err)
	}

	logger.Infof("Creating EventSource")
	k8sSource := test.KubernetesEventSource(eventSource, defaultNamespaceName, testNamespace, serviceAccount, test.ChannelRef(channelName))
	err = CreateKubernetesEventSource(clients, k8sSource, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create KubernetesEventSource: %v", err)
	}

	logger.Infof("Creating Route and Config")
	// The receiver of events which is accessible through Route
	configImagePath := ImagePath("k8sevents")
	err = WithRouteReady(clients, logger, cleaner, routeName, configImagePath)
	if err != nil {
		t.Fatalf("The Route was not marked as Ready to serve traffic: %v", err)
	}

	logger.Infof("Creating Subscription")
	subscription := test.Subscription(subscriptionName, defaultNamespaceName, test.ChannelRef(channelName), test.SubscriberSpecForRoute(routeName), nil)
	err = CreateSubscription(clients, subscription, logger, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Subscription: %v", err)
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
