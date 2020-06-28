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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
)

/*
EventTransformationForTriggerTestHelper tests the following scenario:

                         5                 4
                   ------------- ----------------------
                   |           | |                    |
             1     v	 2     | v        3           |
EventSource ---> Broker ---> Trigger1 -------> Service(Transformation)
                   |
                   | 6                   7
                   |-------> Trigger2 -------> Service(Logger)

Note: the number denotes the sequence of the event that flows in this test case.
*/
func EventTransformationForTriggerTestHelper(t *testing.T,
	brokerClass string,
	brokerVersion string,
	triggerVersion string,
	componentsTestRunner testlib.ComponentsTestRunner,
	options ...testlib.SetupClientOption) {
	const (
		senderName = "e2e-eventtransformation-sender"
		brokerName = "e2e-eventtransformation-broker"

		eventType              = "type1"
		transformedEventType   = "type2"
		eventSource            = "source1"
		transformedEventSource = "source2"
		eventBody              = `{"msg":"e2e-eventtransformation-body"}`
		transformedBody        = `{"msg":"transformed body"}`

		originalTriggerName    = "trigger1"
		transformedTriggerName = "trigger2"

		transformationPodName = "trans-pod"
		recordEventsPodName   = "recordevents-pod"
	)

	componentsTestRunner.RunTests(t, testlib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		client := testlib.Setup(st, true, options...)
		defer testlib.TearDown(client)

		// Create a configmap used by the broker.
		config := client.CreateBrokerConfigMapOrFail(brokerName, &channel)

		// create a new broker
		if brokerVersion == "v1" {
			client.CreateBrokerV1OrFail(brokerName, resources.WithBrokerClassForBrokerV1(brokerClass), resources.WithConfigForBrokerV1(config))
		} else {
			client.CreateBrokerV1Beta1OrFail(brokerName, resources.WithBrokerClassForBrokerV1Beta1(brokerClass), resources.WithConfigForBrokerV1Beta1(config))
		}
		client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

		// create the transformation service
		transformationPod := resources.EventTransformationPod(
			transformationPodName,
			transformedEventType,
			transformedEventSource,
			[]byte(transformedBody),
		)
		client.CreatePodOrFail(transformationPod, testlib.WithService(transformationPodName))

		// create trigger1 for event transformation
		if triggerVersion == "v1" {
			client.CreateTriggerV1OrFail(
				originalTriggerName,
				resources.WithBrokerV1(brokerName),
				resources.WithAttributesTriggerFilterV1(eventSource, eventType, nil),
				resources.WithSubscriberServiceRefForTriggerV1(transformationPodName),
			)
		} else {
			client.CreateTriggerOrFailV1Beta1(
				originalTriggerName,
				resources.WithBrokerV1Beta1(brokerName),
				resources.WithAttributesTriggerFilterV1Beta1(eventSource, eventType, nil),
				resources.WithSubscriberServiceRefForTriggerV1Beta1(transformationPodName),
			)
		}

		// create logger pod and service
		eventTracker, _ := recordevents.StartEventRecordOrFail(client, recordEventsPodName)
		defer eventTracker.Cleanup()

		// create trigger2 for event receiving
		if triggerVersion == "v1" {
			client.CreateTriggerV1OrFail(
				transformedTriggerName,
				resources.WithBrokerV1(brokerName),
				resources.WithAttributesTriggerFilterV1(transformedEventSource, transformedEventType, nil),
				resources.WithSubscriberServiceRefForTriggerV1(recordEventsPodName),
			)
		} else {
			client.CreateTriggerOrFailV1Beta1(
				transformedTriggerName,
				resources.WithBrokerV1Beta1(brokerName),
				resources.WithAttributesTriggerFilterV1Beta1(transformedEventSource, transformedEventType, nil),
				resources.WithSubscriberServiceRefForTriggerV1Beta1(recordEventsPodName),
			)
		}

		// wait for all test resources to be ready, so that we can start sending events
		client.WaitForAllTestResourcesReadyOrFail()

		// eventToSend is the event sent as input of the test
		eventToSend := cloudevents.NewEvent()
		eventToSend.SetID(uuid.New().String())
		eventToSend.SetType(eventType)
		eventToSend.SetSource(eventSource)
		if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}
		client.SendEventToAddressable(senderName, brokerName, testlib.BrokerTypeMeta, eventToSend)

		// check if the logging service receives the correct event
		eventTracker.AssertAtLeast(1, recordevents.MatchEvent(
			HasSource(transformedEventSource),
			HasType(transformedEventType),
			HasData([]byte(transformedBody)),
		))

		eventTracker.AssertNot(recordevents.MatchEvent(
			HasSource(eventSource),
			HasType(eventType),
			HasData([]byte(eventBody)),
		))
	})
}
