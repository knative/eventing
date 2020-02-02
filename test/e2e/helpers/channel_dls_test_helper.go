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

// ChannelDeadLetterSinkTestHelper is the helper function for channel_deadlettersink_test
func ChannelDeadLetterSinkTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName    = "e2e-channelchain-sender"
		loggerPodName = "e2e-channel-dls-logger-pod"
	)
	channelNames := []string{"e2e-channel-dls"}
	// subscriptionNames corresponds to Subscriptions
	subscriptionNames := []string{"e2e-channel-dls-subs1"}

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create channels
		client.CreateChannelsOrFail(channelNames, &channel)
		client.WaitForResourcesReadyOrFail(&channel)

		// create loggerPod and expose it as a service
		pod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

		// create subscriptions that subscribe to a service that does not exist
		client.CreateSubscriptionsOrFail(
			subscriptionNames,
			channelNames[0],
			&channel,
			resources.WithSubscriberForSubscription("does-not-exist"),
			resources.WithDeadLetterSinkForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the first channel
		body := fmt.Sprintf("TestChannelDeadLetterSink %s", uuid.NewUUID())
		event := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, body),
			cloudevents.WithSource(senderName),
		)
		client.SendFakeEventToAddressableOrFail(senderName, channelNames[0], &channel, event)

		// check if the logging service receives the correct number of event messages
		expectedContentCount := len(subscriptionNames)
		if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(body, expectedContentCount)); err != nil {
			st.Fatalf("String %q does not appear %d times in logs of logger pod %q: %v", body, expectedContentCount, loggerPodName, err)
		}
	})
}
