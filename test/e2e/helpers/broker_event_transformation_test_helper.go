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

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// EventTransformationForTriggerTestHelper is the helper function for broker_event_tranformation_test
func EventTransformationForTriggerTestHelper(t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName = "e2e-eventtransformation-sender"
		brokerName = "e2e-eventtransformation-broker"

		any          = v1alpha1.TriggerAnyFilter
		eventType1   = "type1"
		eventType2   = "type2"
		eventSource1 = "source1"
		eventSource2 = "source2"
		eventBody    = "e2e-eventtransformation-body"

		triggerName1 = "trigger1"
		triggerName2 = "trigger2"

		transformationPodName = "trans-pod"
		loggerPodName         = "logger-pod"
	)

	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := lib.Setup(st, true, options...)
		defer lib.TearDown(client)

		// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
		client.CreateRBACResourcesForBrokers()

		// create a new broker
		client.CreateBrokerOrFail(brokerName, resources.WithChannelTemplateForBroker(&channel))
		client.WaitForResourceReadyOrFail(brokerName, lib.BrokerTypeMeta)

		// create the event we want to transform to
		transformedEventBody := fmt.Sprintf("%s %s", eventBody, string(uuid.NewUUID()))
		eventAfterTransformation := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, transformedEventBody),
			cloudevents.WithSource(eventSource2),
			cloudevents.WithType(eventType2),
		)

		// create the transformation service
		transformationPod := resources.EventTransformationPod(transformationPodName, eventAfterTransformation)
		client.CreatePodOrFail(transformationPod, lib.WithService(transformationPodName))

		// create trigger1 for event transformation
		client.CreateTriggerOrFail(
			triggerName1,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource1, eventType1),
			resources.WithSubscriberServiceRefForTrigger(transformationPodName),
		)

		// create logger pod and service
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		// create trigger2 for event receiving
		client.CreateTriggerOrFail(
			triggerName2,
			resources.WithBroker(brokerName),
			resources.WithDeprecatedSourceAndTypeTriggerFilter(eventSource2, eventType2),
			resources.WithSubscriberServiceRefForTrigger(loggerPodName),
		)

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// send fake CloudEvent to the broker
		eventToSend := cloudevents.New(
			fmt.Sprintf(`{"msg":%q}`, eventBody),
			cloudevents.WithSource(eventSource1),
			cloudevents.WithType(eventType1),
		)
		client.SendFakeEventToAddressableOrFail(senderName, brokerName, lib.BrokerTypeMeta, eventToSend)

		// check if the logging service receives the correct event
		if err := client.CheckLog(loggerPodName, lib.CheckerContains(transformedEventBody)); err != nil {
			st.Fatalf("String %q not found in logs of logger pod %q: %v", transformedEventBody, loggerPodName, err)
		}
	})
}
