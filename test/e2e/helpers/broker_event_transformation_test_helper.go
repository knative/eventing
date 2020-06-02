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

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// EventTransformationForTriggerTestHelper is the helper function for broker_event_tranformation_test
func EventTransformationForTriggerTestHelper(t *testing.T,
	brokerClass string,
	channelTestRunner lib.ChannelTestRunner,
	options ...lib.SetupClientOption) {
	const (
		senderName = "e2e-eventtransformation-sender"
		brokerName = "e2e-eventtransformation-broker"

		any          = v1beta1.TriggerAnyFilter
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

		// Create a configmap used by the broker.
		config := client.CreateBrokerConfigMapOrFail(brokerName, &channel)

		// create a new broker
		client.CreateBrokerV1Beta1OrFail(brokerName, resources.WithBrokerClassForBrokerV1Beta1(brokerClass), resources.WithConfigForBrokerV1Beta1(config))
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
		client.CreateTriggerOrFailV1Beta1(
			triggerName1,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(eventSource1, eventType1, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(transformationPodName),
		)

		// create logger pod and service
		loggerPod := resources.EventLoggerPod(loggerPodName)
		client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

		// create trigger2 for event receiving
		client.CreateTriggerOrFailV1Beta1(
			triggerName2,
			resources.WithBrokerV1Beta1(brokerName),
			resources.WithAttributesTriggerFilterV1Beta1(eventSource2, eventType2, nil),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
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
