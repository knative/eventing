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
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
)

// BrokerDeadLetterSinkTestHelper is the helper function for broker_dls_test
func BrokerDeadLetterSinkTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	const (
		senderName = "e2e-brokerchannel-sender"
		brokerName = "e2e-brokerchannel-broker"

		eventType   = "type"
		eventSource = "source"
		eventBody   = "e2e-brokerchannel-body"

		triggerName = "e2e-brokerchannel-trigger"

		loggerPodName = "e2e-brokerchannel-logger-pod"

		channelName      = "e2e-brokerchannel-channel"
		subscriptionName = "e2e-brokerchannel-subscription"
	)

	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := common.Setup(st, true)
		defer common.TearDown(client)

		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateRBACResourcesForBrokers()

		// create logger pod and service for deadlettersink
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, common.WithService(loggerPodName))

		delivery := resources.Delivery(resources.WithDeadLetterSinkForDelivery(loggerPodName))

		// create a new broker
		client.CreateBrokerOrFail(brokerName,
			resources.WithChannelTemplateForBroker(&channel),
			resources.WithDeliveryForBroker(delivery))

		client.WaitForResourceReady(brokerName, common.BrokerTypeMeta)

		// create trigger to receive the original event, and send to an invalid destination
		client.CreateTriggerOrFail(
			triggerName,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource, eventType),
			resources.WithSubscriberURIForTrigger("http://does-not-exist.svc.cluster.local"),
		)

		// wait for all test resources to be ready, so that we can start sending events
		if err := client.WaitForAllTestResourcesReady(); err != nil {
			st.Fatalf("Failed to get all test resources ready: %v", err)
		}

		// send fake CloudEvent to the broker
		eventToSend := &resources.CloudEvent{
			Source:   eventSource,
			Type:     eventType,
			Data:     fmt.Sprintf(`{"msg":%q}`, eventBody),
			Encoding: resources.CloudEventDefaultEncoding,
		}

		if err := client.SendFakeEventToAddressable(senderName, brokerName, common.BrokerTypeMeta, eventToSend); err != nil {
			st.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
		}

		// check if deadlettersink logging service received event
		if err := client.CheckLog(loggerPodName, common.CheckerContains(eventBody)); err != nil {
			st.Fatalf("Strings %v not found in logs of logger pod %q: %v", eventBody, loggerPodName, err)
		}

	})
}
