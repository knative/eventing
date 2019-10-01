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

func BrokerTracingTestHelper(t *testing.T, channelTestRunner common.ChannelTestRunner) {
	testCases := map[string]struct {
		incomingTraceId bool
		istio           bool
	}{
		"includes incoming trace id": {
			incomingTraceId: true,
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

				// Label namespace so that it creates the default broker.
				if err := client.LabelNamespace(map[string]string{"knative-eventing-injection": "enabled"}); err != nil {
					st.Fatalf("Error annotating namespace: %v", err)
				}

				// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
				// TestMain.
				tracinghelper.Setup(st, client)

				expected, mustContain := setupBrokerTracing(st, channel, client, loggerPodName, tc.incomingTraceId)
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
// 1. Broker.
// 2. Trigger on 'foo' events -> K8s Service -> transformer Pod (which replies with a 'bar' event).
// 3. Trigger on 'bar' events -> K8s Service -> eventdetails Pod.
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func setupBrokerTracing(t *testing.T, channel string, client *common.Client, loggerPodName string, incomingTraceId bool) (tracinghelper.TestSpanTree, string) {
	// Create the Broker.
	const (
		brokerName    = "br"
		etTransformer = "transformer"
		etLogger      = "logger"
	)
	channelTypeMeta := common.GetChannelTypeMeta(channel)
	client.CreateBrokerOrFail(brokerName, channelTypeMeta)

	// DO NOT SUBMIT!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!111!!!!!!!!!!!!!!!!!!
	time.Sleep(10 * time.Second)

	// Create an logger (EventDetails) Pod and a K8s Service that points to it.
	logPod := resources.EventDetailsPod(loggerPodName)
	client.CreatePodOrFail(logPod, common.WithService(loggerPodName))

	// Create a Trigger that receive events (type=bar) and send to logger Pod.
	loggerTrigger := client.CreateTriggerOrFail(
		"logger",
		resources.WithBroker(brokerName),
		resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// Create an event mutator to response an event with type bar
	eventTransformerPod := resources.EventTransformationPod("transformer", &resources.CloudEvent{
		Type: etLogger,
	})
	client.CreatePodOrFail(eventTransformerPod, common.WithService(eventTransformerPod.Name))

	// Create a Trigger that receive events (type=foo) and send to event mutator Pod.
	transformerTrigger := client.CreateTriggerOrFail(
		"transformer",
		resources.WithBroker(brokerName),
		resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
		resources.WithSubscriberRefForTrigger(eventTransformerPod.Name),
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
		Type:     etTransformer,
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

	// TODO Actually determine the cluster's domain, similar to knative.dev/pkg/network/domain.go.
	domain := "cluster.local"

	// We expect the following spans:
	// 0. Artificial root span.
	// 1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).
	// 2. Broker Ingress receives the event from the sending pod.
	// 3. Broker Ingress sends the event to the Broker's TrChannel (trigger channel).
	// 4. Broker TrChannel receives the event from the Broker Ingress.
	// 5. Broker TrChannel sends the event to the Broker Filter for the "logger" trigger.
	//     6. Broker Filter for the "logger" trigger receives the event from the Broker TrChannel.
	//        This does not pass the filter, so this 'branch' ends here.
	// 7. Broker TrChannel sends the event to the Broker Filter for the "transformer" trigger.
	// 8. Broker Filter for the "transformer" trigger receives the event from the Broker TrChannel.
	// 9. Broker Filter for the "transformer" trigger sends the event to the transformer pod.
	// 10. Transformer pod receives the event from the Broker Filter for the "transformer" trigger.
	// 11. Broker Filter for the "transformer" sends the transformer pod's reply to the Broker
	//     InChannel (Ingress Channel).
	// 12. Broker InChannel receives the event from the Broker Filter for the "transformer" trigger.
	// 13. Broker InChannel sends the event to the Broker Ingress.
	// 14. Broker Ingress receives the event from the Broker InChannel.
	// 15. Broker Ingress sends the event to the Broker's TrChannel.
	// 16. Broker TrChannel receives the event from the Broker Ingress.
	// 17. Broker TrChannel sends the event to the Broker Filter for the "transformer" trigger.
	//     18. Broker Filter for the "transformer" trigger receives the event from the Broker
	//        TrChannel. This does not pass the filter, so this 'branch' ends here.
	// 19. Broker TrChannel sends the event to the Broker Filter for the "logger" trigger.
	// 20. Broker Filter for the "logger" trigger receives the event from the Broker TrChannel.
	// 21. Broker Filter for the "logger" trigger sends the event to the logger pod.
	// 22. Logger pod receives the event from the Broker Filter for the "logger" trigger.

	// Useful constants we will use below.
	ingressHost := fmt.Sprintf("%s-broker.%s.svc.%s", brokerName, client.Namespace, domain)
	triggerChanHost := fmt.Sprintf("%s-kne-trigger.%s.svc.%s", brokerName, client.Namespace, domain)
	ingressChanHost := fmt.Sprintf("%s-kne-ingress.%s.svc.%s", brokerName, client.Namespace, domain)
	filterHost := fmt.Sprintf("%s-broker-filter.%s.svc.%s", brokerName, client.Namespace, domain)
	loggerTriggerPath := fmt.Sprintf("/triggers/%s/%s/%s", client.Namespace, loggerTrigger.Name, loggerTrigger.UID)
	transformerTriggerPath := fmt.Sprintf("/triggers/%s/%s/%s", client.Namespace, transformerTrigger.Name, transformerTrigger.UID)
	loggerSVCHost := fmt.Sprintf("%s.%s.svc.%s", loggerPodName, client.Namespace, domain)
	transformerSVCHost := fmt.Sprintf("%s.%s.svc.%s", eventTransformerPod.Name, client.Namespace, domain)

	// This is very hard to read when written directly, so we will build piece by piece.

	// Steps 17-18: 'logger' event being sent to the 'transformer' Trigger.
	loggerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
		// 17. Broker TrChannel sends the event to the Broker Filter for the "transformer" trigger.
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      "POST",
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/%s", filterHost, transformerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				// 18. Broker Filter for the "transformer" trigger receives the event from the
				// Broker TrChannel. This does not pass the filter, so this 'branch' ends here.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        transformerTriggerPath,
				},
			},
		},
	}

	// Steps 19-22: 'logger' event being sent to the 'logger' Trigger.
	loggerEventSentFromTrChannelToLogger := tracinghelper.TestSpanTree{
		// 19. Broker TrChannel sends the event to the Broker Filter for the "logger" trigger.
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      "POST",
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/%s", filterHost, loggerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				// 20. Broker Filter for the "logger" trigger receives the event from the Broker TrChannel.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        transformerTriggerPath,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						// 21. Broker Filter for the "logger" trigger sends the event to the logger pod.
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      "POST",
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", loggerSVCHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								// 22. Logger pod receives the event from the Broker Filter for the "logger" trigger.
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      "POST",
									"http.path":        "/",
									"http.status_code": "202",
									"http.host":        fmt.Sprintf("unknown-foobar="),
								},
							},
						},
					},
				},
			},
		},
	}

	// Steps 13-22. Directly steps 13-16. 17-22 are included as children.
	// Steps 13-16: Event in the Broker InChannel sent to the Broker Ingress to the Trigger Channel.
	loggerEventIngressToTrigger := tracinghelper.TestSpanTree{
		// 13. Broker InChannel sends the event to the Broker Ingress.
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      "POST",
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/", ingressHost),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				// 14. Broker Ingress receives the event from the Broker InChannel.
				Kind: model.Server,
				Tags: map[string]string{

					"http.method":      "POST",
					"http.path":        "/",
					"http.status_code": "202",
					"http.host":        ingressHost,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						// 15. Broker Ingress sends the event to the Broker's TrChannel.
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      "POST",
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s", triggerChanHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								// 16. Broker TrChannel receives the event from the Broker Ingress.
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      "POST",
									"http.status_code": "202",
									"http.host":        triggerChanHost,
									"http.path":        "/",
								},
								Children: []tracinghelper.TestSpanTree{
									// Steps 17-18.
									loggerEventSentFromTrChannelToTransformer,
									// Steps 19-22.
									loggerEventSentFromTrChannelToLogger,
								},
							},
						},
					},
				},
			},
		},
	}

	// Steps 7-22. Directly steps 7-12. 13-22 are included as children.
	// Steps 7-12: Event from TrChannel sent to transformer Trigger and its reply to the InChannel.
	transformerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
		// 7. Broker TrChannel sends the event to the Broker Filter for the "transformer" trigger.
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      "POST",
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/%s", filterHost, transformerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				// 8. Broker Filter for the "transformer" trigger receives the event from the Broker
				// TrChannel.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        transformerTriggerPath,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						// 9. Broker Filter for the "transformer" trigger sends the event to the
						// transformer pod.
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      "POST",
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", transformerSVCHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								// 10. Transformer pod receives the event from the Broker Filter for
								// the "transformer" trigger.
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      "POST",
									"http.path":        "/",
									"http.status_code": "202",
									"http.host":        fmt.Sprintf("unknown-bazqux="),
								},
								Children: []tracinghelper.TestSpanTree{
									{
										// 11. Broker Filter for the "transformer" sends the
										// transformer pod's reply to the Broker InChannel.
										Kind: model.Client,
										Tags: map[string]string{
											"http.method":      "POST",
											"http.status_code": "202",
											"http.url":         fmt.Sprintf("http://%s", ingressChanHost),
										},
										Children: []tracinghelper.TestSpanTree{
											{
												// 12. Broker InChannel receives the event from the
												// Broker Filter for the "transformer" trigger.
												Kind: model.Server,
												Tags: map[string]string{
													"http.method":      "POST",
													"http.status_code": "202",
													"http.url":         ingressChanHost,
													"http.path":        "/",
												},
												Children: []tracinghelper.TestSpanTree{
													// Steps 13-22.
													loggerEventIngressToTrigger,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Steps 5-6:  Event from TrChannel sent to logger Trigger.
	transformerEventSentFromTrChannelToLogger := tracinghelper.TestSpanTree{
		// 5. Broker TrChannel sends the event to the Broker Filter for the "logger" trigger.
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      "POST",
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/%s", filterHost, loggerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				// 6. Broker Filter for the "logger" trigger receives the event from the Broker
				// TrChannel. This does not pass the filter, so this 'branch' ends here.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        loggerTriggerPath,
				},
			},
		},
	}

	// 0. Artificial root span.
	// 1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).
	// 2. Broker Ingress receives the event from the sending pod.
	// 3. Broker Ingress sends the event to the Broker's TrChannel (trigger channel).
	// 4. Broker TrChannel receives the event from the Broker Ingress.

	// Steps 0-22. Directly steps 0-4 (missing 1)
	// Steps 0-4 (missing 1, which is optional and added below if present): Event sent to the Broker
	// Ingress.
	expected := tracinghelper.TestSpanTree{
		// 0. Artificial root span.
		Root: true,
		Children: []tracinghelper.TestSpanTree{
			{
				// 2. Broker Ingress receives the event from the sending pod.
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.host":        ingressHost,
					"http.path":        "/",
				},
				Children: []tracinghelper.TestSpanTree{
					{
						// 3. Broker Ingress sends the event to the Broker's TrChannel (trigger channel).
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      "POST",
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s", triggerChanHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								// 4. Broker TrChannel receives the event from the Broker Ingress.
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      "POST",
									"http.status_code": "202",
									"http.host":        triggerChanHost,
									"http.path":        "/",
								},
								Children: []tracinghelper.TestSpanTree{
									// Steps 5-6.
									transformerEventSentFromTrChannelToLogger,
									// Steps 7-22.
									transformerEventSentFromTrChannelToTransformer,
								},
							},
						},
					},
				},
			},
		},
	}

	if incomingTraceId {
		expected.Children = []tracinghelper.TestSpanTree{
			{
				// 1. Send pod sends event to the Broker Ingress (only if the sending pod generates
				// a span).
				Kind:                     model.Client,
				LocalEndpointServiceName: "sender",
				Tags: map[string]string{
					"http.method":      "POST",
					"http.status_code": "202",
					"http.url":         fmt.Sprintf("http://%s-broker.%s.svc.cluster.local", brokerName, client.Namespace),
				},
				Children: expected.Children,
			},
		}
	}
	return expected, body
}
