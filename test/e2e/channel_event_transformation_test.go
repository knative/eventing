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

	"github.com/knative/eventing/test/base/resources"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

/*
TestEventTransformationForSubscriptiop tests the following scenario:

             1            2                 5            6                  7
EventSource ---> Channel ---> Subscription ---> Channel ---> Subscription ----> Service(Logger)
                                   |  ^
                                 3 |  | 4
                                   |  |
                                   |  ---------
                                   -----------> Service(Transformation)
*/
func TestEventTransformationForSubscription(t *testing.T) {
	senderName := "e2e-eventtransformation-sender"
	channelNames := []string{"e2e-eventtransformation1", "e2e-eventtransformation2"}
	// subscriptionNames1 corresponds to Subscriptions on channelNames[0]
	subscriptionNames1 := []string{"e2e-eventtransformation-subs11", "e2e-eventtransformation-subs12"}
	// subscriptionNames2 corresponds to Subscriptions on channelNames[1]
	subscriptionNames2 := []string{"e2e-eventtransformation-subs21", "e2e-eventtransformation-subs22"}
	transformationPodName := "e2e-eventtransformation-transformation-pod"
	loggerPodName := "e2e-eventtransformation-logger-pod"

	runTests(t, provisioners, common.FeatureBasic, func(st *testing.T, provisioner string, isCRD bool) {
		client := setup(st, true)
		defer tearDown(client)

		// create channels
		channelTypeMeta := getChannelTypeMeta(provisioner, isCRD)
		client.CreateChannelsOrFail(channelNames, channelTypeMeta, provisioner)
		client.WaitForResourcesReady(channelTypeMeta)

		// create transformation pod and service
		transformedEventBody := fmt.Sprintf("eventBody %s", uuid.NewUUID())
		eventAfterTransformation := &resources.CloudEvent{
			Source:   senderName,
			Type:     resources.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
		client.CreatePodOrFail(transformationPod, common.WithService(transformationPodName))

		// create logger pod and service
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

		// create subscriptions that subscribe the first channel, use the transformation service to transform the events and then forward the transformed events to the second channel
		client.CreateSubscriptionsOrFail(
			subscriptionNames1,
			channelNames[0],
			channelTypeMeta,
			resources.WithSubscriberForSubscription(transformationPodName),
			resources.WithReplyForSubscription(channelNames[1], channelTypeMeta),
		)
		// create subscriptions that subscribe the second channel, and forward the received events to the logger service
		client.CreateSubscriptionsOrFail(
			subscriptionNames2,
			channelNames[1],
			channelTypeMeta,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the first channel
		eventBody := fmt.Sprintf("TestEventTransformation %s", uuid.NewUUID())
		eventToSend := &resources.CloudEvent{
			Source:   senderName,
			Type:     resources.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
			Encoding: resources.CloudEventDefaultEncoding,
		}
		if err := client.SendFakeEventToAddressable(senderName, channelNames[0], channelTypeMeta, eventToSend); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelNames[0])
		}

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames1) * len(subscriptionNames2)
		if err := client.CheckLog(loggerPodName, common.CheckerContainsCount(transformedEventBody, expectedContentCount)); err != nil {
			st.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", transformedEventBody, expectedContentCount, loggerPodName, err)
		}
	})
}
