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

package helpers

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

/*
singleEvent tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/

// SingleEventHelperForChannelTestHelper is the helper function for header_test
func SingleEventHelperForChannelTestHelper(t *testing.T, encoding string,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption,
) {
	channelName := "conformance-headers-channel-" + encoding
	senderName := "conformance-headers-sender-" + encoding
	subscriptionName := "conformance-headers-subscription-" + encoding
	loggerPodName := "conformance-headers-logger-pod-" + encoding

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		st.Logf("Running header conformance test with channel %q", channel)
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channel
		st.Logf("Creating channel")
		client.CreateChannelOrFail(channelName, &channel)

		// create logger service as the subscriber
		pod := resources.EventDetailsPod(loggerPodName)
		client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			&channel,
			resources.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the channel
		eventID := fmt.Sprintf("%s", uuid.NewUUID())
		body := fmt.Sprintf("TestSingleHeaderEvent %s", eventID)
		event := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, body),
			cloudevents.WithSource(senderName),
			cloudevents.WithID(eventID),
			cloudevents.WithEncoding(encoding),
		)
		st.Logf("Sending event with tracing headers to %s", senderName)
		client.SendFakeEventWithTracingToAddressableOrFail(senderName, channelName, &channel, event)

		// verify the logger service receives the event
		st.Logf("Logging for event with body %s", body)

		if err := client.CheckLog(loggerPodName, lib.CheckerContains(body)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
		}

		// verify that required x-b3-spani and x-b3-traceid are set
		requiredHeaderNameList := []string{"X-B3-Traceid", "X-B3-Spanid", "X-B3-Sampled"}
		for _, headerName := range requiredHeaderNameList {
			expectedHeaderLog := fmt.Sprintf("Got Header %s:", headerName)
			if err := client.CheckLog(loggerPodName, lib.CheckerContains(expectedHeaderLog)); err != nil {
				st.Fatalf("String %q not found in logs of logger pod %q: %v", expectedHeaderLog, loggerPodName, err)
			}
		}

		//TODO report on optional x-b3-parentspanid and x-b3-sampled if present?
		//TODO report x-custom-header

	})
}
