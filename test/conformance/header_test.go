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

package conformance

import (
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

const (
	userHeaderKey   = "this-was-user-set"
	userHeaderValue = "a value"
)

// The Channel MUST pass through all tracing information as CloudEvents attributes
func TestMustPassTracingHeaders(t *testing.T) {
	singleEvent(t, resources.CloudEventEncodingBinary)
}

/*
singleEvent tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/
func singleEvent(t *testing.T, encoding string) {
	channelName := "conformance-headers-channel-" + encoding
	senderName := "conformance-headers-sender-" + encoding
	subscriptionName := "conformance-headers-subscription-" + encoding
	loggerPodName := "conformance-headers-logger-pod-" + encoding

	runTests(t, channels, common.FeatureBasic, func(st *testing.T, channel string) {
		st.Logf("Running header conformance test with channel %q", channel)
		client := setup(st, true)
		defer tearDown(client)

		// create channel
		st.Logf("Creating channel")
		channelTypeMeta := getChannelTypeMeta(channel)
		client.CreateChannelOrFail(channelName, channelTypeMeta)

		// create logger service as the subscriber
		pod := resources.EventDetailsPod(loggerPodName)
		client.CreatePodOrFail(pod, common.WithService(loggerPodName))

		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			channelTypeMeta,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the channel
		body := fmt.Sprintf("TestSingleHeaderEvent %s", uuid.NewUUID())
		event := &resources.CloudEvent{
			Source:   senderName,
			Type:     resources.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: encoding,
		}

		st.Logf("Sending event to %s", senderName)
		if err := client.SendFakeEventToAddressable(senderName, channelName, channelTypeMeta, event); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelName)
		}

		// verify the logger service receives the event
		st.Logf("Logging for event with body %s", body)

		if err := client.CheckLog(loggerPodName, common.CheckerContains(body)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
		}
		//TODO verify that x-b3-spani and x-b3-traceid are set
		//TODO report on optional x-b3-parentspanid and x-b3-sampled if present?
	})
}
