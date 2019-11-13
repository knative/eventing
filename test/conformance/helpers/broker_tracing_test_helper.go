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
	"net/http"
	"testing"

	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
)

// BrokerTracingTestHelperWithChannelTestRunner runs the Broker tracing tests for all Channels in
// the ChannelTestRunner.
func BrokerTracingTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner common.ChannelTestRunner,
	setupClient SetupClientFunc,
) {
	channelTestRunner.RunTests(t, common.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		// Don't accidentally use t, use st instead. To ensure this, shadow 't' to a useless type.
		t := struct{}{}
		_ = fmt.Sprintf("%s", t)

		BrokerTracingTestHelper(st, channel, setupClient)
	})
}

// BrokerTracingTestHelper runs the Broker tracing test using the given TypeMeta.
func BrokerTracingTestHelper(t *testing.T, channel metav1.TypeMeta, setupClient SetupClientFunc) {
	testCases := map[string]TracingTestCase{
		"includes incoming trace id": {
			IncomingTraceId: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tracingTest(t, setupClient, setupBrokerTracing, channel, tc)
		})
	}
}

// setupBrokerTracing is the general setup for TestBrokerTracing. It creates the following:
// 1. Broker.
// 2. Trigger on 'foo' events -> K8s Service -> transformer Pod (which replies with a 'bar' event).
// 3. Trigger on 'bar' events -> K8s Service -> eventdetails Pod.
// 4. Sender Pod which sends a 'foo' event.
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func setupBrokerTracing(
	t *testing.T,
	channel *metav1.TypeMeta,
	client *common.Client,
	loggerPodName string,
	tc TracingTestCase,
) (tracinghelper.TestSpanTree, string) {
	const (
		etTransformer = "transformer"
		etLogger      = "logger"
	)
	// Create the Broker.
	client.CreateRBACResourcesForBrokers()
	broker := client.CreateBrokerOrFail("br", channel)

	// Create a logger (EventDetails) Pod and a K8s Service that points to it.
	logPod := resources.EventDetailsPod(loggerPodName)
	client.CreatePodOrFail(logPod, common.WithService(loggerPodName))

	// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
	loggerTrigger := client.CreateTriggerOrFail(
		"logger",
		resources.WithBroker(broker.Name),
		resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberRefForTrigger(loggerPodName),
	)

	// Create a transformer (EventTransfrmer) Pod that replies with the same event as the input,
	// except the reply's event's type is changed to bar.
	eventTransformerPod := resources.EventTransformationPod("transformer", &resources.CloudEvent{
		Type: etLogger,
	})
	client.CreatePodOrFail(eventTransformerPod, common.WithService(eventTransformerPod.Name))

	// Create a Trigger that receives events (type=foo) and sends them to the transformer Pod.
	transformerTrigger := client.CreateTriggerOrFail(
		"transformer",
		resources.WithBroker(broker.Name),
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
	if tc.IncomingTraceId {
		sendEvent = client.SendFakeEventWithTracingToAddressable
	}
	if err := sendEvent(senderName, broker.Name, common.BrokerTypeMeta, event); err != nil {
		t.Fatalf("Failed to send fake CloudEvent to the broker %q", broker.Name)
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
	ingressHost := brokerIngressHost(domain, *broker)
	ingressChanHost := brokerIngressChannelHost(domain, *broker)
	triggerChanHost := brokerTriggerChannelHost(domain, *broker)
	filterHost := brokerFilterHost(domain, *broker)
	loggerTriggerPath := triggerPath(*loggerTrigger)
	transformerTriggerPath := triggerPath(*transformerTrigger)
	loggerSVCHost := k8sServiceHost(domain, client.Namespace, loggerPodName)
	transformerSVCHost := k8sServiceHost(domain, client.Namespace, eventTransformerPod.Name)

	// This is very hard to read when written directly, so we will build piece by piece.

	// Steps 17-18: 'logger' event being sent to the 'transformer' Trigger.
	loggerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
		Note: "17. Broker TrChannel sends the event to the Broker Filter for the 'transformer' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s%s", filterHost, transformerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "18. Broker Filter for the 'transformer' trigger receives the event from the Broker TrChannel. This does not pass the filter, so this 'branch' ends here.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        transformerTriggerPath,
				},
			},
		},
	}

	// Steps 19-22: 'logger' event being sent to the 'logger' Trigger.
	loggerEventSentFromTrChannelToLogger := tracinghelper.TestSpanTree{
		Note: "19. Broker TrChannel sends the event to the Broker Filter for the 'logger' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s%s", filterHost, loggerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "20. Broker Filter for the 'logger' trigger receives the event from the Broker TrChannel.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        loggerTriggerPath,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "21. Broker Filter for the 'logger' trigger sends the event to the logger pod.",
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      http.MethodPost,
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", loggerSVCHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "22. Logger pod receives the event from the Broker Filter for the 'logger' trigger.",
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      http.MethodPost,
									"http.path":        "/",
									"http.status_code": "202",
									"http.host":        loggerSVCHost,
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
		Note: "13. Broker InChannel sends the event to the Broker Ingress.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/", ingressHost),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "14. Broker Ingress receives the event from the Broker InChannel.",
				Kind: model.Server,
				Tags: map[string]string{

					"http.method":      http.MethodPost,
					"http.path":        "/",
					"http.status_code": "202",
					"http.host":        ingressHost,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "15. Broker Ingress sends the event to the Broker's TrChannel.",
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      http.MethodPost,
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", triggerChanHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "16. Broker TrChannel receives the event from the Broker Ingress.",
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      http.MethodPost,
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

	// Steps 7-10: Event from TrChannel sent to transformer Trigger and its reply to the InChannel.
	transformerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
		Note: "7. Broker TrChannel sends the event to the Broker Filter for the 'transformer' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method": http.MethodPost,
			"http.url":    fmt.Sprintf("http://%s%s", filterHost, transformerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "8. Broker Filter for the 'transformer' trigger receives the event from the Broker TrChannel.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        transformerTriggerPath,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "9. Broker Filter for the 'transformer' trigger sends the event to the transformer pod.",
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      http.MethodPost,
							"http.status_code": "200",
							"http.url":         fmt.Sprintf("http://%s/", transformerSVCHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "10. Transformer pod receives the event from the Broker Filter for the 'transformer' trigger.",
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      http.MethodPost,
									"http.path":        "/",
									"http.status_code": "200",
									"http.host":        transformerSVCHost,
								},
							},
						},
					},
				},
			},
		},
	}

	// Step 11-22. Directly steps 11-12. Steps 13-22 are children.
	// Steps 11-12 Reply from the 'transformer' is sent by the Broker TrChannel to the Broker
	// InChannel.
	transformerEventResponseFromTrChannel := tracinghelper.TestSpanTree{
		Note: "11. Broker TrChannel for the 'transformer' sends the transformer pod's reply to the Broker InChannel.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s", ingressChanHost),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "12. Broker InChannel receives the event from the Broker TrChannel for the 'transformer' trigger.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        ingressChanHost,
					"http.path":        "/",
				},
				Children: []tracinghelper.TestSpanTree{
					// Steps 13-22.
					loggerEventIngressToTrigger,
				},
			},
		},
	}

	// Steps 5-6: Event from TrChannel sent to logger Trigger.
	transformerEventSentFromTrChannelToLogger := tracinghelper.TestSpanTree{
		Note: "5. Broker TrChannel sends the event to the Broker Filter for the 'logger' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s%s", filterHost, loggerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "6. Broker Filter for the 'logger' trigger receives the event from the Broker TrChannel. This does not pass the filter, so this 'branch' ends here.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        loggerTriggerPath,
				},
			},
		},
	}

	// Steps 0-22. Directly steps 0-4 (missing 1).
	// Steps 0-4 (missing 1, which is optional and added below if present): Event sent to the Broker
	// Ingress.
	expected := tracinghelper.TestSpanTree{
		Note: "0. Artificial root span.",
		Root: true,
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "2. Broker Ingress receives the event from the sending pod.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        ingressHost,
					"http.path":        "/",
				},
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "3. Broker Ingress sends the event to the Broker's TrChannel (trigger channel).",
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      http.MethodPost,
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", triggerChanHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "4. Broker TrChannel receives the event from the Broker Ingress.",
								Kind: model.Server,
								Tags: map[string]string{
									"http.method":      http.MethodPost,
									"http.status_code": "202",
									"http.host":        triggerChanHost,
									"http.path":        "/",
								},
								Children: []tracinghelper.TestSpanTree{
									// Steps 5-6.
									transformerEventSentFromTrChannelToLogger,
									// Steps 7-10.
									transformerEventSentFromTrChannelToTransformer,
									// Steps 11-22
									transformerEventResponseFromTrChannel,
								},
							},
						},
					},
				},
			},
		},
	}

	if tc.IncomingTraceId {
		expected.Children = []tracinghelper.TestSpanTree{
			{
				Note:                     "1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).",
				Kind:                     model.Client,
				LocalEndpointServiceName: "sender",
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.url":         fmt.Sprintf("http://%s", ingressHost),
				},
				Children: expected.Children,
			},
		}
	}
	return expected, body
}
