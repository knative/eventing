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

	"github.com/openzipkin/zipkin-go/model"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	"knative.dev/pkg/test/zipkin"
)

// BrokerTracingTestHelper is to run broker tracing test
func BrokerTracingTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	testCases := map[string]struct {
		incomingTraceID bool
		istio           bool
	}{
		"includes incoming trace id": {
			incomingTraceID: true,
		},
	}

	for n, tc := range testCases {
		loggerPodName := "logger"
		t.Run(n, func(t *testing.T) {
			channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel string) {
				// Don't accidentally use t, use st instead. To ensure this, shadow 't' to a useless
				// type.
				t := struct{}{}
				_ = fmt.Sprintf("%s", t)

				client := common.Setup(st, true)
				defer common.TearDown(client)

				// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
				// TestMain.
				tracinghelper.Setup(st, client)

				expected, mustContain := setupBrokerTracing(st, channel, client, loggerPodName, tc.incomingTraceID)
				assertLogContents(st, client, loggerPodName, mustContain)

				traceID := getTraceID(st, client, loggerPodName)
				trace, err := zipkin.JSONTrace(traceID, expected.SpanCount(), 60*time.Second)
				if err != nil {
					st.Fatalf("Unable to get trace %q: %v. Trace so far %+v", traceID, err, tracinghelper.PrettyPrintTrace(trace))
				}
				st.Logf("I got the trace, %q!\n%+v", traceID, tracinghelper.PrettyPrintTrace(trace))

				tree := tracinghelper.GetTraceTree(st, trace)
				if err := expected.Matches(tree); err != nil {
					st.Fatalf("Trace Tree did not match expected: %v", err)
				}
			})
		})
	}
}

// setupBrokerTracing is the general setup for TestBrokerTracing. It creates the following:
// SendEvents (Pod) -> Broker -> Subscription -> K8s Service -> LogEvents (Pod)
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func setupBrokerTracing(t *testing.T, channel string, client *common.Client, loggerPodName string, incomingTraceId bool) (tracinghelper.TestSpanTree, string) {
	// Create the Broker.
	const (
		brokerName     = "br"
		eventTypeFoo   = "foo"
		eventTypeBar   = "bar"
		mutatorPodName = "mutator"
		any            = v1alpha1.TriggerAnyFilter
	)
	channelTypeMeta := common.GetChannelTypeMeta(channel)
	client.CreateRBACResourcesForBrokers()
	client.CreateBrokerOrFail(brokerName, channelTypeMeta)
	client.WaitForResourceReady(brokerName, common.BrokerTypeMeta)

	// Create a LogEvents Pod and a K8s Service that points to it.
	logPod := resources.EventDetailsPod(loggerPodName)
	client.CreatePodOrFail(logPod, common.WithService(loggerPodName))

	// Create a Trigger that receive events (type=bar) and send to logger Pod.
	client.CreateTriggerOrFail(
		"trigger-bar",
		resources.WithBroker(brokerName),
		resources.WithDeprecatedSourceAndTypeTriggerFilter(any, eventTypeBar),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// Create an event mutator to response an event with type bar
	eventMutatorPod := resources.EventMutatorPod(mutatorPodName, eventTypeBar)
	client.CreatePodOrFail(eventMutatorPod, common.WithService(mutatorPodName))

	// Create a Trigger that receive events (type=foo) and send to event mutator Pod.
	client.CreateTriggerOrFail(
		"trigger-foo",
		resources.WithBroker(brokerName),
		resources.WithDeprecatedSourceAndTypeTriggerFilter(any, eventTypeFoo),
		resources.WithSubscriberRefForTrigger(mutatorPodName),
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

	// We expect the following spans:
	// 0. Artificial root span.
	// 1. EventSender Pod to Broker Ingress.
	// 2. Broker Ingress to Trigger Channel.
	// 3. Trigger Channel to Trigger 1.
	// 4. Trigger Channel to Trigger 2.
	// 5. Trigger 1 to event mutator.
	// 6. Trigger 1 response to Ingress Channel.
	// 7. Ingress Channel to Broker Ingress.
	// 8. Broker Ingress to Trigger Channel.
	// 9. Trigger Channel to Trigger 1.
	// 10.Trigger Channel to Trigger 2.
	// 11. Trigger 2 to logger.
	expected := tracinghelper.TestSpanTree{
		// 0. Artificial root span.
		Root: true,
		// 1 is added below if it is needed.
		Children: []tracinghelper.TestSpanTree{
			{
				// 2. Channel receives event from sending pod.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", channel, client.Namespace),
					"http.path":        "/",
				},
				Children: []tracinghelper.TestSpanTree{
					{
						// 3. Channel sends event to logging pod.
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      "POST",
							"http.path":        "/",
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s.%s.svc.cluster.local/", logPod.Name, client.Namespace),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								// 4. Logging pod receives event from Channel.
								Kind:                     model.Server,
								LocalEndpointServiceName: "logger",
								Tags: map[string]string{
									"http.method":      "POST",
									"http.path":        "/",
									"http.status_code": "202",
									"http.host":        fmt.Sprintf("%s.%s.svc.cluster.local", logPod.Name, client.Namespace),
								},
							},
						},
					},
				},
			},
		},
	}

	// if incomingTraceId {
	// 	expected.Children = []tracinghelper.TestSpanTree{
	// 		{
	// 			// 1. Sending pod sends event to Channel (only if the sending pod generates a span).
	// 			Kind:                     model.Client,
	// 			LocalEndpointServiceName: "sender",
	// 			Tags: map[string]string{
	// 				"http.method":      "POST",
	// 				"http.status_code": "202",
	// 				"http.url":         fmt.Sprintf("http://%s-kn-channel.%s.svc.cluster.local", channelName, client.Namespace),
	// 			},
	// 			Children: expected.Children,
	// 		},
	// 	}
	// }
	return expected, body
}
