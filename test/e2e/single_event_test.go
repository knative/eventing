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

	"github.com/knative/eventing/test"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	channelName      = "e2e-singleevent"
	provisionerName  = "in-memory-channel"
	subscriberName   = "e2e-singleevent-subscriber"
	senderName       = "e2e-singleevent-sender"
	subscriptionName = "e2e-singleevent-subscription"
	routeName        = "e2e-singleevent-route"
)

func namespaceExists(t *testing.T, clients *test.Clients) (string, func()) {
	logger := logging.GetContextLogger("TestSingleEvent")
	shutdown := func() {}
	ns := pkgTest.Flags.Namespace
	logger.Infof("Namespace: %s", ns)

	nsSpec, err := clients.Kube.Kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{})

	if err != nil && errors.IsNotFound(err) {
		nsSpec = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
		logger.Infof("Creating Namespace: %s", ns)
		nsSpec, err = clients.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)
		if err != nil {
			t.Fatalf("Failed to create Namespace: %s; %v", ns, err)
		} else {
			shutdown = func() {
				clients.Kube.Kube.CoreV1().Namespaces().Delete(nsSpec.Name, nil)
				// TODO: this is a bit hacky but in order for the tests to work
				// correctly for a clean namespace to be created we need to also
				// wait for it to be removed.
				// To fix this we could generate namespace names.
				// This only happens when the namespace provided does not exist.
				//
				// wait up to 120 seconds for the namespace to be removed.
				logger.Infof("Deleting Namespace: %s", ns)
				for i := 0; i < 120; i++ {
					time.Sleep(1 * time.Second)
					if _, err := clients.Kube.Kube.CoreV1().Namespaces().Get(ns, metav1.GetOptions{}); err != nil && errors.IsNotFound(err) {
						logger.Info("Namespace has been deleted")
						// the namespace is gone.
						break
					}
				}
			}
		}
	}
	return ns, shutdown
}

func TestSingleBinaryEvent(t *testing.T) {
	SingleEvent(t, test.CloudEventEncodingBinary)
}

func TestSingleStructuredEvent(t *testing.T) {
	SingleEvent(t, test.CloudEventEncodingStructured)
}

func SingleEvent(t *testing.T, encoding string) {
	logger := logging.GetContextLogger("TestSingleEvent")

	clients, cleaner := Setup(t, logger)
	defer TearDown(clients, cleaner, logger)

	// verify namespace

	ns, cleanupNS := namespaceExists(t, clients)
	defer cleanupNS()

	// create logger pod

	logger.Infof("creating subscriber pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	subscriberPod := test.EventLoggerPod(routeName, ns, selector)
	if err := CreatePod(clients, subscriberPod, logger, cleaner); err != nil {
		t.Fatalf("Failed to create event logger pod: %v", err)
	}
	if err := WaitForAllPodsRunning(clients, logger, ns); err != nil {
		t.Fatalf("Error waiting for logger pod to become running: %v", err)
	}
	logger.Infof("subscriber pod running")

	subscriberSvc := test.Service(routeName, ns, selector)
	if err := CreateService(clients, subscriberSvc, logger, cleaner); err != nil {
		t.Fatalf("Failed to create event logger service: %v", err)
	}

	// Reload subscriberPod to get IP
	subscriberPod, err := clients.Kube.Kube.CoreV1().Pods(subscriberPod.Namespace).Get(subscriberPod.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get subscriber pod: %v", err)
	}

	// create channel

	logger.Infof("Creating Channel and Subscription")
	channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(provisionerName))
	logger.Infof("channel: %#v", channel)
	sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelName), test.SubscriberSpecForService(routeName), nil)
	logger.Infof("sub: %#v", sub)

	if err := WithChannelAndSubscriptionReady(clients, channel, sub, logger, cleaner); err != nil {
		t.Fatalf("The Channel or Subscription were not marked as Ready: %v", err)
	}

	// create sender pod

	logger.Infof("Creating event sender")
	body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
	event := test.CloudEvent{
		Source:   senderName,
		Type:     "test.eventing.knative.dev",
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: encoding,
	}
	url := fmt.Sprintf("http://%s", channel.Status.Address.Hostname)
	pod := test.EventSenderPod(senderName, ns, url, event)
	logger.Infof("sender pod: %#v", pod)
	if err := CreatePod(clients, pod, logger, cleaner); err != nil {
		t.Fatalf("Failed to create event sender pod: %v", err)
	}

	if err := WaitForLogContent(clients, logger, routeName, subscriberPod.Spec.Containers[0].Name, ns, body); err != nil {
		t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body, routeName, err)
	}
}
