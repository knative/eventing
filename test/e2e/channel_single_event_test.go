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

	"github.com/knative/eventing/test/base"
	"github.com/knative/eventing/test/common"
	"k8s.io/apimachinery/pkg/util/uuid"
)

func TestSingleBinaryEventForChannel(t *testing.T) {
	singleEvent(t, base.CloudEventEncodingBinary)
}

func TestSingleStructuredEventForChannel(t *testing.T) {
	singleEvent(t, base.CloudEventEncodingStructured)
}

/*
singleEvent tests the following scenario:

EventSource ---> Channel ---> Subscription ---> Service(Logger)

*/
func singleEvent(t *testing.T, encoding string) {
	channelName := "e2e-singleevent-channel-" + encoding
	senderName := "e2e-singleevent-sender-" + encoding
	subscriptionName := "e2e-singleevent-subscription-" + encoding
	loggerPodName := "e2e-singleevent-logger-pod-" + encoding

	RunTests(t, common.FeatureBasic, func(st *testing.T, provisioner string) {
		st.Logf("Run test with provisioner %q", provisioner)
		client := Setup(st, true)
		defer TearDown(client)

		// create channel
		client.CreateChannelOrFail(channelName, provisioner)

		// create logger service as the subscriber
		pod := base.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(pod, common.WithService(loggerPodName))

		// create subscription to subscribe the channel, and forward the received events to the logger service
		client.CreateSubscriptionOrFail(
			subscriptionName,
			channelName,
			base.WithSubscriberForSubscription(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the channel
		body := fmt.Sprintf("TestSingleEvent %s", uuid.NewUUID())
		event := &base.CloudEvent{
			Source:   senderName,
			Type:     base.CloudEventDefaultType,
			Data:     fmt.Sprintf(`{"msg":%q}`, body),
			Encoding: encoding,
		}
		if err := client.SendFakeEventToChannel(senderName, channelName, event); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the channel %q", channelName)
		}

		// verify the logger service receives the event
		if err := client.CheckLog(loggerPodName, common.CheckerContains(body)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
		}
	})
}
