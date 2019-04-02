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
	pkgTest "github.com/knative/pkg/test"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestSingleBinaryEvent(t *testing.T) {
	SingleEvent(t, test.CloudEventEncodingBinary)
}

func TestSingleStructuredEvent(t *testing.T) {
	SingleEvent(t, test.CloudEventEncodingStructured)
}

/*
SingleEvent tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/
func SingleEvent(t *testing.T, encoding string) {
	if test.EventingFlags.Provisioner == "" {
		t.Fatal("ClusterChannelProvisioner must be set to a non-empty string. Either do not specify --clusterChannelProvisioner or set to something other than the empty string")
	}

	const (
		channelName      = "e2e-singleevent"
		subscriberName   = "e2e-singleevent-subscriber"
		senderName       = "e2e-singleevent-sender"
		subscriptionName = "e2e-singleevent-subscription"
		routeName        = "e2e-singleevent-route"
	)

	clients, cleaner := Setup(t, t.Logf)
	// verify namespace
	ns, cleanupNS := CreateNamespaceIfNeeded(t, clients, t.Logf)
	defer cleanupNS()

	// TearDown() needs to be deferred after cleanupNS(). Otherwise the namespace is deleted and all
	// resources in it. So when TearDown() runs, it spews a lot of not found errors.
	defer TearDown(clients, cleaner, t.Logf)

	// create logger pod
	t.Logf("creating subscriber pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	subscriberPod := test.EventLoggerPod(routeName, ns, selector)
	subscriberPod, err := CreatePodAndServiceReady(clients, subscriberPod, routeName, ns, selector, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create subscriber pod and service, and get them ready: %v", err)
	}

	// create channel

	t.Logf("Creating Channel and Subscription")
	channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(test.EventingFlags.Provisioner))
	t.Logf("channel: %#v", channel)
	sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelName), test.SubscriberSpecForService(routeName), nil)
	t.Logf("sub: %#v", sub)

	if err := WithChannelsAndSubscriptionsReady(clients, &[]*v1alpha1.Channel{channel}, &[]*v1alpha1.Subscription{sub}, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channel or Subscription were not marked as Ready: %v", err)
	}

	// send fake CloudEvent to the first channel
	body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
	if err := SendFakeEventToChannel(clients, senderName, body, test.CloudEventDefaultType, test.CloudEventDefaultEncoding, channel, ns, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channel.Name)
	}

	if err := pkgTest.WaitForLogContent(clients.Kube, routeName, subscriberPod.Spec.Containers[0].Name, body); err != nil {
		clients.Kube.PodLogs(senderName, "sendevent")
		clients.Kube.PodLogs(senderName, "istio-proxy")
		t.Fatalf("String %q not found in logs of subscriber pod %q: %v", body, routeName, err)
	}
}
