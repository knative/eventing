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

	ce "github.com/cloudevents/sdk-go"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// BrokerTracingTestHelperWithChannelTestRunner runs the Broker tracing tests for all Channels in
// the ChannelTestRunner.
func BrokerTracingTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	setupClient lib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		// Don't accidentally use t, use st instead. To ensure this, shadow 't' to a useless type.
		t := struct{}{}
		_ = fmt.Sprintf("%s", t)

		BrokerTracingTestHelper(st, channel, setupClient)
	})
}

// BrokerTracingTestHelper runs the Broker tracing test using the given TypeMeta.
func BrokerTracingTestHelper(t *testing.T, channel metav1.TypeMeta, setupClient lib.SetupClientOption) {
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
	client *lib.Client,
	loggerPodName string,
	tc TracingTestCase,
) (tracinghelper.TestSpanTree, string) {
	const (
		etTransformer  = "transformer"
		etLogger       = "logger"
		defaultCMPName = "eventing"
	)
	// Create the Broker.
	client.CreateConfigMapPropagationOrFail(defaultCMPName)
	client.CreateRBACResourcesForBrokers()
	broker := client.CreateBrokerOrFail("br", resources.WithChannelTemplateForBroker(channel))

	// Create a logger (EventDetails) Pod and a K8s Service that points to it.
	logPod := resources.EventDetailsPod(loggerPodName)
	client.CreatePodOrFail(logPod, lib.WithService(loggerPodName))

	// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
	loggerTrigger := client.CreateTriggerOrFail(
		"logger",
		resources.WithBroker(broker.Name),
		resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(loggerPodName),
	)

	// Create a transformer (EventTransfrmer) Pod that replies with the same event as the input,
	// except the reply's event's type is changed to bar.
	eventTransformerPod := resources.EventTransformationPod("transformer", &cloudevents.CloudEvent{
		EventContextV1: ce.EventContextV1{
			Type: etLogger,
		},
	})
	client.CreatePodOrFail(eventTransformerPod, lib.WithService(eventTransformerPod.Name))

	// Create a Trigger that receives events (type=foo) and sends them to the transformer Pod.
	transformerTrigger := client.CreateTriggerOrFail(
		"transformer",
		resources.WithBroker(broker.Name),
		resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
		resources.WithSubscriberServiceRefForTrigger(eventTransformerPod.Name),
	)

	// Wait for all test resources to be ready, so that we can start sending events.
	client.WaitForAllTestResourcesReadyOrFail()

	// Everything is setup to receive an event. Generate a CloudEvent.
	senderName := "sender"
	eventID := fmt.Sprintf("%s", uuid.NewUUID())
	body := fmt.Sprintf("TestBrokerTracing %s", eventID)
	event := cloudevents.New(
		fmt.Sprintf(`{"msg":%q}`, body),
		cloudevents.WithSource(senderName),
		cloudevents.WithID(eventID),
		cloudevents.WithType(etTransformer),
	)

	// Send the CloudEvent (either with or without tracing inside the SendEvents Pod).
	sendEvent := client.SendFakeEventToAddressableOrFail
	if tc.IncomingTraceId {
		sendEvent = client.SendFakeEventWithTracingToAddressableOrFail
	}
	sendEvent(senderName, broker.Name, lib.BrokerTypeMeta, event)

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
	//     Ingress.
	// 12. Broker Ingress receives the event from the Broker Filter for the "transformer" trigger.
	// 13. Broker Ingress sends the event to the Broker's TrChannel.
	// 14. Broker TrChannel receives the event from the Broker Ingress.
	// 15. Broker TrChannel sends the event to the Broker Filter for the "transformer" trigger.
	//     16. Broker Filter for the "transformer" trigger receives the event from the Broker
	//        TrChannel. This does not pass the filter, so this 'branch' ends here.
	// 17. Broker TrChannel sends the event to the Broker Filter for the "logger" trigger.
	// 18. Broker Filter for the "logger" trigger receives the event from the Broker TrChannel.
	// 19. Broker Filter for the "logger" trigger sends the event to the logger pod.
	// 20. Logger pod receives the event from the Broker Filter for the "logger" trigger.

	// Useful constants we will use below.
	ingressHost := brokerIngressHost(domain, *broker)
	triggerChanHost := brokerTriggerChannelHost(domain, *broker)
	filterHost := brokerFilterHost(domain, *broker)
	loggerTriggerPath := triggerPath(*loggerTrigger)
	transformerTriggerPath := triggerPath(*transformerTrigger)
	loggerSVCHost := k8sServiceHost(domain, client.Namespace, loggerPodName)
	transformerSVCHost := k8sServiceHost(domain, client.Namespace, eventTransformerPod.Name)

	// This is very hard to read when written directly, so we will build piece by piece.

	// Steps 15-16: 'logger' event being sent to the 'transformer' Trigger.
	loggerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
		Note: "15. Broker TrChannel sends the event to the Broker Filter for the 'transformer' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s%s", filterHost, transformerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "16. Broker Filter for the 'transformer' trigger receives the event from the Broker TrChannel. This does not pass the filter, so this 'branch' ends here.",
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

	// Steps 17-20: 'logger' event being sent to the 'logger' Trigger.
	loggerEventSentFromTrChannelToLogger := tracinghelper.TestSpanTree{
		Note: "17. Broker TrChannel sends the event to the Broker Filter for the 'logger' trigger.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s%s", filterHost, loggerTriggerPath),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "18. Broker Filter for the 'logger' trigger receives the event from the Broker TrChannel.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        filterHost,
					"http.path":        loggerTriggerPath,
				},
				Children: []tracinghelper.TestSpanTree{
					{
						Note: "19. Broker Filter for the 'logger' trigger sends the event to the logger pod.",
						Kind: model.Client,
						Tags: map[string]string{
							"http.method":      http.MethodPost,
							"http.status_code": "202",
							"http.url":         fmt.Sprintf("http://%s/", loggerSVCHost),
						},
						Children: []tracinghelper.TestSpanTree{
							{
								Note: "20. Logger pod receives the event from the Broker Filter for the 'logger' trigger.",
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

	// Steps 13-20. Directly steps 15-16. 17-20 are included as children.
	loggerEventIngressToTrigger := tracinghelper.TestSpanTree{
		Note: "13. Broker Ingress sends the event to the Broker's TrChannel.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s/", triggerChanHost),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "14. Broker TrChannel receives the event from the Broker Ingress.",
				Kind: model.Server,
				Tags: map[string]string{
					"http.method":      http.MethodPost,
					"http.status_code": "202",
					"http.host":        triggerChanHost,
					"http.path":        "/",
				},
				Children: []tracinghelper.TestSpanTree{
					// Steps 15-16.
					loggerEventSentFromTrChannelToTransformer,
					// Steps 17-20.
					loggerEventSentFromTrChannelToLogger,
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

	// Step 11-20. Directly steps 11-12. Steps 13-20 are children.
	// Steps 11-12 Reply from the 'transformer' is sent by the Broker TrChannel to the Broker
	// Ingress.
	transformerEventResponseFromTrChannel := tracinghelper.TestSpanTree{
		Note: "11. Broker TrChannel for the 'transformer' sends the transformer pod's reply to the Broker Ingress.",
		Kind: model.Client,
		Tags: map[string]string{
			"http.method":      http.MethodPost,
			"http.status_code": "202",
			"http.url":         fmt.Sprintf("http://%s", ingressHost),
		},
		Children: []tracinghelper.TestSpanTree{
			{
				Note: "12. Broker Ingress receives the event from the Broker Filter for the 'transformer' trigger.",
				Kind: model.Server,
				Tags: map[string]string{

					"http.method":      http.MethodPost,
					"http.path":        "/",
					"http.status_code": "202",
					"http.host":        ingressHost,
				},
				Children: []tracinghelper.TestSpanTree{
					// Steps 13-20.
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
