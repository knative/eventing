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
	"context"
	"fmt"
	"testing"

	"github.com/knative/eventing/test"
	"github.com/knative/eventing/test/e2e/broker_trigger/builder"
	pkgTest "github.com/knative/pkg/test"
	"github.com/knative/pkg/test/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

const (
	defaultBrokerName = "default"
	altBrokerName     = "alternate"
	untypedEvent      = "test.untyped"
	typedEvent        = "test.typed"
	unsourcedEvent    = "test.unsourced"
	sourcedEvent      = "test.sourced"
)

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

	// Fixtures

	// TODO label namespace to get default Broker

	fixtures := []Fixture{
		// Default Any Trigger
		&KnativeFixture{
			Object: builder.Trigger("default-dump-any", pkgTest.Flags.Namespace).
				SubscriberSvc("default-any-dumper"),
		},
		// Default Typed Trigger
		&KnativeFixture{
			Object: builder.Trigger("default-dump-typed", pkgTest.Flags.Namespace).
				Type(typedEvent).
				SubscriberSvc("default-typed-dumper"),
		},
		// Default Sourced Trigger
		&KnativeFixture{
			Object: builder.Trigger("default-dump-sourced", pkgTest.Flags.Namespace).
				Type(sourcedEvent).
				SubscriberSvc("default-sourced-dumper"),
		},

		// Alternate Broker
		&KnativeFixture{
			Object: builder.Broker(altBrokerName, pkgTest.Flags.Namespace),
		},
		// Alternate Any Trigger
		&KnativeFixture{
			Object: builder.Trigger("alt-dump-any", pkgTest.Flags.Namespace).
				Broker(altBrokerName).
				SubscriberSvc(fmt.Sprintf("alt-any-dumper")),
		},
		// Alternate Typed Trigger
		&KnativeFixture{
			Object: builder.Trigger("alt-dump-typed", pkgTest.Flags.Namespace).
				Broker(altBrokerName).
				Type(typedEvent).
				SubscriberSvc("alt-typed-dumper"),
		},
		// Alternate Sourced Trigger
		&KnativeFixture{
			Object: builder.Trigger("alt-dump-sourced", pkgTest.Flags.Namespace).
				Broker(altBrokerName).
				Type(sourcedEvent).
				SubscriberSvc("alt-sourced-dumper"),
		},
	}

	ctx := context.Background()

	//TODO move this to runners

	for _, f := range fixtures {
		f.Create(ctx, client)
	}

	for _, f := range fixtures {
		f.Verify(ctx, client)
	}

	// Create message dumper services
	// Create: pod
	// Verify: pod status is ready
	//

	//pods := []Fixture{
	//	&PodSuccess{
	//
	//	}
	//}

	// create alt Broker

	// For each broker, create a typed and untyped trigger.
	// For each trigger, create a logevents pod.
	// Wait for Broker, triggers, and pods to become ready.

	// Take Action

	// For each tuple of typed and untyped, default and alt broker:
	//   create a sendevents pod to send an event of the type to the broker address

	// Verify

	// For each logevents pod:
	//   check logs to ensure the correct message(s) got there

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
	if test.EventingFlags.Provisioner == "" {
		t.Fatal("ClusterChannelProvisioner must be set to a non-empty string. Either do not specify --clusterChannelProvisioner or set to something other than the empty string")
	}
	channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(test.EventingFlags.Provisioner))
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
