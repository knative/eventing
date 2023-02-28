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
	"context"
	"strings"
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
// Deprecated, use reconciler-test based tests.
func EventTransformationForTriggerTestHelper(
	ctx context.Context,
	t *testing.T,
	brokerVersion string,
	triggerVersion string,
	creator BrokerCreator,
	options ...testlib.SetupClientOption) {
	const (
		senderName = "e2e-eventtransformation-sender"

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

	client := testlib.Setup(t, true, options...)
	defer testlib.TearDown(client)

	brokerName := creator(client, brokerVersion)
	client.WaitForResourceReadyOrFail(brokerName, testlib.BrokerTypeMeta)

	// create the transformation service
	recordevents.DeployEventRecordOrFail(
		ctx,
		client,
		transformationPodName,
		recordevents.ReplyWithTransformedEvent(
			transformedEventType,
			transformedEventSource,
			transformedBody,
		),
	)

	// create trigger1 for event transformation
	client.CreateTriggerOrFail(
		originalTriggerName,
		resources.WithBroker(brokerName),
		resources.WithAttributesTriggerFilter(eventSource, eventType, nil),
		resources.WithSubscriberServiceRefForTrigger(transformationPodName),
	)

	// create logger pod and service
	eventTracker, _ := recordevents.StartEventRecordOrFail(ctx, client, recordEventsPodName)
	// create trigger2 for event receiving
	client.CreateTriggerOrFail(
		transformedTriggerName,
		resources.WithBroker(brokerName),
		resources.WithAttributesTriggerFilter(transformedEventSource, transformedEventType, nil),
		resources.WithSubscriberServiceRefForTrigger(recordEventsPodName),
	)

	// wait for all test resources to be ready, so that we can start sending events
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// eventToSend is the event sent as input of the test
	eventToSend := cloudevents.NewEvent()
	eventToSend.SetID(uuid.New().String())
	eventToSend.SetType(eventType)
	eventToSend.SetSource(eventSource)
	if err := eventToSend.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
		t.Fatal("Cannot set the payload of the event:", err.Error())
	}
	client.SendEventToAddressable(ctx, senderName, brokerName, testlib.BrokerTypeMeta, eventToSend)

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
}

// BrokerCreator creates a broker and returns its broker name.
type BrokerCreator func(client *testlib.Client, version string) string

// ChannelBasedBrokerCreator creates a BrokerCreator that creates a broker based on the channel parameter.
func ChannelBasedBrokerCreator(channel metav1.TypeMeta, brokerClass string) BrokerCreator {
	return func(client *testlib.Client, version string) string {
		brokerName := strings.ToLower(channel.Kind)

		// create a ConfigMap used by the broker.
		config := client.CreateBrokerConfigMapOrFail("config-"+brokerName, &channel)

		switch version {
		case "v1":
			client.CreateBrokerOrFail(brokerName,
				resources.WithBrokerClassForBroker(brokerClass),
				resources.WithConfigForBroker(config),
			)
		default:
			panic("unknown version: " + version)
		}

		return brokerName
	}
}
