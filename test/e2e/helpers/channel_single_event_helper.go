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

type subscriptionVersion string

const (
	subscriptionV1alpha1 subscriptionVersion = "v1alpha1"
	subscriptionV1beta1  subscriptionVersion = "v1beta1"
)

// SingleEventForChannelTestHelper is the helper function for channel_single_event_test
func SingleEventForChannelTestHelper(t *testing.T, encoding string,
	subscriptionVersion subscriptionVersion,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	channelName := "e2e-singleevent-channel-" + encoding
	senderName := "e2e-singleevent-sender-" + encoding
	subscriptionName := "e2e-singleevent-subscription-" + encoding
	loggerPodName := "e2e-singleevent-logger-pod-" + encoding

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		st.Logf("Run test with channel %q", channel)
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channel
		client.CreateChannelOrFail(channelName, &channel)

		// create logger service as the subscriber
		pod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

		// create subscription to subscribe the channel, and forward the received events to the logger service
		switch subscriptionVersion {
		case subscriptionV1alpha1:
			client.CreateSubscriptionOrFail(
				subscriptionName,
				channelName,
				&channel,
				resources.WithSubscriberForSubscription(loggerPodName),
			)
		case subscriptionV1beta1:
			client.CreateSubscriptionOrFailV1Beta1(
				subscriptionName,
				channelName,
				&channel,
				resources.WithSubscriberForSubscriptionV1Beta1(loggerPodName),
			)
		}

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the channel
		body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
		event := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, body),
			cloudevents.WithSource(senderName),
			cloudevents.WithEncoding(encoding),
		)
		client.SendFakeEventToAddressableOrFail(senderName, channelName, &channel, event)

		// verify the logger service receives the event
		if err := client.CheckLog(loggerPodName, lib.CheckerContains(body)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
		}
	})
}
