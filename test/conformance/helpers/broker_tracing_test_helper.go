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
	"time"

	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	"knative.dev/pkg/test/zipkin"
)

func BrokerTracingTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	testCases := map[string]struct {
		incomingTraceId bool
		istio           bool
	}{
		"no incoming trace id": {},
		"includes incoming trace id": {
			incomingTraceId: true,
		}, /*
			"caller has istio": {
				istio: true,
			},*/
	}

	for n, tc := range testCases {
		loggerPodName := "logger"
		t.Run(n, func(t *testing.T) {
			channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
				// Don't accidentally use t, use st instead. To ensure this, shadow 't' to some a
				// useless type.
				t := struct{}{}
				_ = fmt.Sprintf("%s", t)

				client := common.Setup(st, true)
				defer common.TearDown(client)

				// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
				// TestMain.
				tracinghelper.Setup(st, client)

				mustContain := setupBrokerTracing(st, channel, client, loggerPodName, tc.incomingTraceId)
				assertLogContents(st, client, loggerPodName, mustContain)

				traceID := getTraceID(st, client, loggerPodName)
				expectedNumSpans := 11
				if tc.incomingTraceId {
					expectedNumSpans = 12
				}
				trace, err := zipkin.JSONTrace(traceID, expectedNumSpans, 15*time.Second)
				if err != nil {
					st.Fatalf("Unable to get trace %q: %v", traceID, err)
				}
				st.Logf("I got the trace, %q!\n%+v", traceID, trace)

				// TODO: Assert information in the trace.
			})
		})
	}
}

// setupBrokerTracing is the general setup for TestBrokerTracing. It creates the following:
// SendEvents (Pod) -> Broker -> Subscription -> K8s Service -> LogEvents (Pod)
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func setupBrokerTracing(t *testing.T, channel string, client *common.Client, loggerPodName string, incomingTraceId bool) string {
	// Create the Broker.
	const (
		brokerName     = "br"
		eventTypeFoo   = "foo"
		eventTypeBar   = "bar"
		mutatorPodName = "mutator"
		any            = v1alpha1.TriggerAnyFilter
	)
	channelTypeMeta := common.GetChannelTypeMeta(channel)
	client.CreateBrokerOrFail(brokerName, channelTypeMeta)

	// Create a LogEvents Pod and a K8s Service that points to it.
	logPod := resources.EventDetailsPod(loggerPodName)
	client.CreatePodOrFail(logPod, common.WithService(loggerPodName))

	// Create a Trigger that receive events (type=bar) and send to logger Pod.
	client.CreateTriggerOrFail(
		"trigger-trace",
		resources.WithBroker(brokerName),
		resources.WithDeprecatedSourceAndTypeTriggerFilter(any, eventTypeBar),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// Create an event mutator to response an event with type bar
	eventMutatorPod := resources.EventMutatorPod(mutatorPodName, eventTypeBar)
	client.CreatePodOrFail(eventMutatorPod, common.WithService(mutatorPodName))

	// Create a Trigger that receive events (type=foo) and send to event mutator Pod.
	client.CreateTriggerOrFail(
		"trigger-trace",
		resources.WithBroker(brokerName),
		resources.WithDeprecatedSourceAndTypeTriggerFilter(any, eventTypeBar),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// Wait for all test resources to be ready, so that we can start sending events.
	if err := client.WaitForAllTestResourcesReady(); err != nil {
		t.Fatalf("Failed to get all test resources ready: %v", err)
	}

	// Everything is setup to receive an event. Generate a CloudEvent.
	senderName := "sender"
	eventID := fmt.Sprintf("%s", uuid.NewUUID())
	body := fmt.Sprintf("TestBrokerTracing %s", eventID)
	event := &resources.CloudEvent{
		ID:       eventID,
		Source:   senderName,
		Type:     eventTypeFoo,
		Data:     fmt.Sprintf(`{"msg":%q}`, body),
		Encoding: resources.CloudEventEncodingBinary,
	}

	// Send the CloudEvent (either with or without tracing inside the SendEvents Pod).
	sendEvent := client.SendFakeEventToAddressable
	if incomingTraceId {
		sendEvent = client.SendFakeEventWithTracingToAddressable
	}
	if err := sendEvent(senderName, brokerName, common.BrokerTypeMeta, event); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", brokerName)
	}
	return body
}
