/*
Copyright 2020 The Knative Authors
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

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// BrokerDeadLetterSinkTestHelper is the helper function for broker_dls_test
func BrokerDeadLetterSinkTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName = "e2e-brokerchannel-sender"
		brokerName = "e2e-brokerchannel-broker"

		eventType   = "type"
		eventSource = "source"
		eventBody   = "e2e-brokerchannel-body"

		triggerName = "e2e-brokerchannel-trigger"

		loggerPodName = "e2e-brokerchannel-logger-pod"
	)

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateRBACResourcesForBrokers()

		// create logger pod and service for deadlettersink
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		delivery := resources.Delivery(resources.WithDeadLetterSinkForDelivery(loggerPodName))

		// create a new broker
		client.CreateBrokerOrFail(brokerName,
			resources.WithChannelTemplateForBroker(&channel),
			resources.WithDeliveryForBroker(delivery))

		client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)

		// create trigger to receive the original event, and send to an invalid destination
		client.CreateTriggerOrFail(
			triggerName,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource, eventType),
			resources.WithSubscriberURIForTrigger("http://does-not-exist.svc.cluster.local"),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the broker
		eventToSend := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, eventBody),
			cloudevents.WithSource(eventSource),
			cloudevents.WithType(eventType),
		)
		client.SendFakeEventToAddressableOrFail(senderName, brokerName, lib.BrokerTypeMeta, eventToSend)

		// check if deadlettersink logging service received event
		if err := client.CheckLog(loggerPodName, lib.CheckerContains(eventBody)); err != nil {
			st.Fatalf("Strings %v not found in logs of logger pod %q: %v", eventBody, loggerPodName, err)
		}

	})
}
