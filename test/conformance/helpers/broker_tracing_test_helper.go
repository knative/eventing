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
	"strings"
	"testing"

	ce "github.com/cloudevents/sdk-go"
	ce2 "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/utils"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// BrokerTracingTestHelperWithChannelTestRunner runs the Broker tracing tests for all Channels in
// the ChannelTestRunner.
func BrokerTracingTestHelperWithChannelTestRunner(
	t *testing.T,
	brokerClass string,
	channelTestRunner lib.ChannelTestRunner,
	setupClient lib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, lib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		BrokerTracingTestHelper(t, brokerClass, channel, setupClient)
	})
}

// BrokerTracingTestHelper runs the Broker tracing test using the given TypeMeta.
func BrokerTracingTestHelper(t *testing.T, brokerClass string, channel metav1.TypeMeta, setupClient lib.SetupClientOption) {
	testCases := map[string]TracingTestCase{
		"includes incoming trace id": {
			IncomingTraceId: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tracingTest(t, setupClient, setupBrokerTracing(brokerClass), channel, tc)
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
func setupBrokerTracing(brokerClass string) SetupInfrastructureFunc {
	const (
		etTransformer  = "transformer"
		etLogger       = "logger"
		defaultCMPName = "eventing"
	)
	return func(
		t *testing.T,
		channel *metav1.TypeMeta,
		client *lib.Client,
		loggerPodName string,
		tc TracingTestCase,
	) (tracinghelper.TestSpanTree, cetest.EventMatcher) {
		// Create a configmap used by the broker.
		client.CreateBrokerConfigMapOrFail("br", channel)

		broker := client.CreateBrokerV1Beta1OrFail(
			"br",
			resources.WithBrokerClassForBrokerV1Beta1(brokerClass),
			resources.WithConfigMapForBrokerConfig(),
		)

		// Create a logger (EventRecord) Pod and a K8s Service that points to it.
		logPod := resources.EventRecordPod(loggerPodName)
		client.CreatePodOrFail(logPod, lib.WithService(loggerPodName))

		// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
		loggerTrigger := client.CreateTriggerOrFailV1Beta1(
			"logger",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
		)

		// Create a transformer (EventTransfrmer) Pod that replies with the same event as the input,
		// except the reply's event's type is changed to bar.
		eventTransformerPod := resources.DeprecatedEventTransformationPod("transformer", &cloudevents.CloudEvent{
			EventContextV1: ce.EventContextV1{
				Type: etLogger,
			},
		})
		client.CreatePodOrFail(eventTransformerPod, lib.WithService(eventTransformerPod.Name))

		// Create a Trigger that receives events (type=foo) and sends them to the transformer Pod.
		transformerTrigger := client.CreateTriggerOrFailV1Beta1(
			"transformer",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(eventTransformerPod.Name),
		)

		// Wait for all test resources to be ready, so that we can start sending events.
		client.WaitForAllTestResourcesReadyOrFail()

		// Everything is setup to receive an event. Generate a CloudEvent.
		senderName := "sender"
		eventID := string(uuid.NewUUID())
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

		domain := utils.GetClusterDomainName()

		// We expect the following spans:
		// 1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).
		// 2. Broker Ingress receives the event from the sending pod.
		// 3. Broker Filter for the "transformer" trigger sends the event to the transformer pod.
		// 4. Transformer pod receives the event from the Broker Filter for the "transformer" trigger.
		// 5. Broker Filter for the "logger" trigger sends the event to the logger pod.
		// 6. Logger pod receives the event from the Broker Filter for the "logger" trigger.

		// Useful constants we will use below.
		loggerSVCHost := k8sServiceHost(domain, client.Namespace, loggerPodName)
		transformerSVCHost := k8sServiceHost(domain, client.Namespace, eventTransformerPod.Name)

		transformerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
			Note: "3. Broker Filter for the 'transformer' trigger sends the event to the transformer pod.",
			Span: triggerSpan(transformerTrigger, eventID),
			Children: []tracinghelper.TestSpanTree{
				{
					Note: "4. Transformer pod receives the event from the Broker Filter for the 'transformer' trigger.",
					Span: tracinghelper.MatchHTTPSpanWithReply(
						model.Server,
						tracinghelper.WithHTTPHostAndPath(transformerSVCHost, "/"),
						tracinghelper.WithLocalEndpointServiceName(eventTransformerPod.Name),
					),
				},
			},
		}

		transformerEventResponseFromTrChannel := tracinghelper.TestSpanTree{
			Note: "5. Broker ingress for reply from the 'transformer'",
			Span: ingressSpan(broker, eventID),
			Children: []tracinghelper.TestSpanTree{
				{
					Note: "6. Broker Filter for the 'logger' trigger sends the event to the logger pod.",
					Span: triggerSpan(loggerTrigger, eventID),
					Children: []tracinghelper.TestSpanTree{
						{
							Note: "7. Logger pod receives the event from the Broker Filter for the 'logger' trigger.",
							Span: tracinghelper.MatchHTTPSpanNoReply(
								model.Server,
								tracinghelper.WithHTTPHostAndPath(loggerSVCHost, "/"),
								tracinghelper.WithLocalEndpointServiceName(loggerPodName),
							),
						},
					},
				},
			},
		}

		// Steps 0-22. Directly steps 0-4 (missing 1).
		// Steps 0-4 (missing 1, which is optional and added below if present): Event sent to the Broker
		// Ingress.
		expected := tracinghelper.TestSpanTree{
			Note: "2. Broker Ingress receives the event from the sending pod.",
			Span: ingressSpan(broker, eventID),
			Children: []tracinghelper.TestSpanTree{
				// Steps 7-10.
				transformerEventSentFromTrChannelToTransformer,
				// Steps 11-22
				transformerEventResponseFromTrChannel,
			},
		}

		if tc.IncomingTraceId {
			expected = tracinghelper.TestSpanTree{
				Note: "1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).",
				Span: tracinghelper.MatchHTTPSpanNoReply(
					model.Client,
					tracinghelper.WithLocalEndpointServiceName(senderName),
				),
				Children: []tracinghelper.TestSpanTree{expected},
			}
		}
		matchFunc := func(ev ce2.Event) error {
			if ev.Source() != senderName {
				return fmt.Errorf("expected source %s, saw %s", senderName, ev.Source())
			}
			if ev.ID() != eventID {
				return fmt.Errorf("expected id %s, saw %s", eventID, ev.ID())
			}
			db := ev.Data()
			if !strings.Contains(string(db), body) {
				return fmt.Errorf("expected substring %s in data %s", body, string(db))
			}
			return nil
		}

		return expected, matchFunc
	}
}

func ingressSpan(broker *v1beta1.Broker, eventID string) *tracinghelper.SpanMatcher {
	return &tracinghelper.SpanMatcher{
		Tags: map[string]string{
			"messaging.system":      "knative",
			"messaging.destination": fmt.Sprintf("broker:%s.%s", broker.Name, broker.Namespace),
			"messaging.message_id":  eventID,
		},
	}
}

func triggerSpan(trigger *v1beta1.Trigger, eventID string) *tracinghelper.SpanMatcher {
	return &tracinghelper.SpanMatcher{
		Tags: map[string]string{
			"messaging.system":      "knative",
			"messaging.destination": fmt.Sprintf("trigger:%s.%s", trigger.Name, trigger.Namespace),
			"messaging.message_id":  eventID,
		},
	}
}
