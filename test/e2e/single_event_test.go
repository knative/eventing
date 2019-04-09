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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	t.Parallel()

	channelName := "e2e-singleevent-" + encoding
	senderName := "e2e-singleevent-sender-" + encoding
	subscriptionName := "e2e-singleevent-subscription-" + encoding
	loggerPodName := "e2e-singleevent-logger-pod-" + encoding

	clients, cleaner := Setup(t, t.Logf)
	defer TearDown(clients, cleaner, t.Logf)

	ns := test.DefaultTestNamespace
	t.Logf(">>>>>>>>>namespace: %s", ns)
	nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	nsSpec, _ = clients.Kube.Kube.CoreV1().Namespaces().Create(nsSpec)
	// create logger pod
	t.Logf("creating logger pod")
	selector := map[string]string{"e2etest": string(uuid.NewUUID())}
	loggerPod := test.EventLoggerPod(loggerPodName, ns, selector)
	loggerSvc := test.Service(loggerPodName, ns, selector)
	loggerPod, err := CreatePodAndServiceReady(clients, loggerPod, loggerSvc, ns, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create logger pod and service, and get them ready: %v", err)
	}

	// create channel and subscription
	t.Logf("Creating Channel and Subscription")
	channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(test.EventingFlags.Provisioner))
	t.Logf("channel: %#v", channel)
	sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelName), test.SubscriberSpecForService(loggerPodName), nil)
	t.Logf("sub: %#v", sub)

	if err := WithChannelsAndSubscriptionsReady(clients, &[]*v1alpha1.Channel{channel}, &[]*v1alpha1.Subscription{sub}, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channel or Subscription were not marked as Ready: %v", err)
	}

	// send fake CloudEvent to the channel
	body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
	event := &test.CloudEvent{
		Source:   senderName,
		Type:     test.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: encoding,
	}
	if err := SendFakeEventToChannel(clients, event, channel, ns, t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channel.Name)
	}

	if err := pkgTest.WaitForLogContent(clients.Kube, loggerPodName, loggerPod.Spec.Containers[0].Name, body); err != nil {
		clients.Kube.PodLogs(senderName, "sendevent")
		clients.Kube.PodLogs(senderName, "istio-proxy")
		t.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
	}
}
