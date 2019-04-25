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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestEventTransformation tests the following scenario:

             1            2                 5            6                  7
EventSource ---> Channel ---> Subscription ---> Channel ---> Subscription ----> Service(Logger)
                                   |  ^
                                 3 |  | 4
                                   |  |
                                   |  ---------
                                   -----------> Service(Transformation)
*/
func TestEventTransformation(t *testing.T) {
	senderName := "e2e-eventtransformation-sender"
	channelNames := [2]string{"e2e-eventtransformation1", "e2e-eventtransformation2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-eventtransformation-subs11", "e2e-eventtransformation-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-eventtransformation-subs21", "e2e-eventtransformation-subs22"}
	transformationPodName := "e2e-eventtransformation-transformation-pod"
	loggerPodName := "e2e-eventtransformation-logger-pod"
	eventBody := fmt.Sprintf("TestEventTransformation %s", uuid.NewUUID())

	clients, ns, provisioner, cleaner := Setup(t, true, t.Logf)
	defer TearDown(clients, ns, cleaner, t.Logf)

	// create subscriberPods and expose them as services
	t.Logf("creating subscriber pods")
	subscriberPods := make([]*corev1.Pod, 0)

	// create transformation pod and service
	transformedEventBody := fmt.Sprintf("eventBody %s", uuid.NewUUID())
	eventAfterTransformation := &test.CloudEvent{
		Source:   senderName,
		Type:     test.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
		Encoding: test.CloudEventDefaultEncoding,
	}
	transformationPodSelector := map[string]string{"e2etest": string(uuid.NewUUID())}
	transformationPod := test.EventTransformationPod(transformationPodName, ns, transformationPodSelector, eventAfterTransformation)
	transformationSvc := test.Service(transformationPodName, ns, transformationPodSelector)
	transformationPod, err := CreatePodAndServiceReady(clients, transformationPod, transformationSvc, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create transformation pod and service, and get them ready: %v", err)
	}
	subscriberPods = append(subscriberPods, transformationPod)
	// create logger pod and service
	loggerPodSelector := map[string]string{"e2etest": string(uuid.NewUUID())}
	loggerPod := test.EventLoggerPod(loggerPodName, ns, loggerPodSelector)
	loggerSvc := test.Service(loggerPodName, ns, loggerPodSelector)
	loggerPod, err = CreatePodAndServiceReady(clients, loggerPod, loggerSvc, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create logger pod and service, and get them ready: %v", err)
	}
	subscriberPods = append(subscriberPods, loggerPod)

	// create channels
	t.Logf("Creating Channel and Subscription")
	channels := make([]*v1alpha1.Channel, 0)
	for _, channelName := range channelNames {
		channel := test.Channel(channelName, ns, test.ClusterChannelProvisioner(provisioner))
		channels = append(channels, channel)
	}

	// create subscriptions
	subs := make([]*v1alpha1.Subscription, 0)
	// create subscriptions that subscribe the first channel, use the transformation service to transform the events and then forward the transformed events to the second channel
	for _, subscriptionName := range subscriptionNames1 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[0]), test.SubscriberSpecForService(transformationPodName), test.ReplyStrategyForChannel(channelNames[1]))
		subs = append(subs, sub)
	}
	// create subscriptions that subscribe the second channel, and call the logging service
	for _, subscriptionName := range subscriptionNames2 {
		sub := test.Subscription(subscriptionName, ns, test.ChannelRef(channelNames[1]), test.SubscriberSpecForService(loggerPodName), nil)
		subs = append(subs, sub)
	}

	// wait for all channels and subscriptions to become ready
	if err := WithChannelsAndSubscriptionsReady(clients, ns, &channels, &subs, t.Logf, cleaner); err != nil {
		t.Fatalf("The Channels or Subscription were not marked as Ready: %v", err)
	}

	eventToSend := &test.CloudEvent{
		Source:   senderName,
		Type:     test.CloudEventDefaultType,
		Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
		Encoding: test.CloudEventDefaultEncoding,
	}
	// send fake CloudEvent to the first channel
	if err := SendFakeEventToChannel(clients, eventToSend, channels[0], t.Logf, cleaner); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the channel %q", channels[0].Name)
	}

	// check if the logging service receives the correct number of event messages
	expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
	loggerContainerName := loggerPod.Spec.Containers[0].Name
	if err := WaitForLogContentCount(clients, loggerPodName, loggerContainerName, ns, transformedEventBody, expectedContentCount); err != nil {
		logPodLogsForDebugging(clients, transformationPodName, transformationPod.Spec.Containers[0].Name, ns, t.Logf)
		logPodLogsForDebugging(clients, loggerPodName, loggerContainerName, ns, t.Logf)
		logPodLogsForDebugging(clients, eventSource1, "sendevent", ns, t.Logf)
		t.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", transformedEventBody, expectedContentCount, loggerPod.Name, err)
	}
}
