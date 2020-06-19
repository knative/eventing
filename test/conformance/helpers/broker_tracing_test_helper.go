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
	"fmt"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1beta1"
	"knative.dev/eventing/pkg/utils"
	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"
)

// BrokerTracingTestHelperWithChannelTestRunner runs the Broker tracing tests for all Channels in
// the ComponentsTestRunner.
func BrokerTracingTestHelperWithChannelTestRunner(
	ctx context.Context,
	t *testing.T,
	brokerClass string,
	channelTestRunner testlib.ComponentsTestRunner,
	setupClient testlib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		tracingTest(ctx, t, setupClient, setupBrokerTracing(ctx, brokerClass), channel)
	})
}

// setupBrokerTracing is the general setup for TestBrokerTracing. It creates the following:
// 1. Broker.
// 2. Trigger on 'foo' events -> K8s Service -> transformer Pod (which replies with a 'bar' event).
// 3. Trigger on 'bar' events -> K8s Service -> eventdetails Pod.
// 4. Sender Pod which sends a 'foo' event.
// It returns a string that is expected to be sent by the SendEvents Pod and should be present in
// the LogEvents Pod logs.
func setupBrokerTracing(ctx context.Context, brokerClass string) SetupTracingTestInfrastructureFunc {
	const (
		etTransformer = "transformer"
		etLogger      = "logger"
		senderName    = "sender"
		eventID       = "event-1"
		eventBody     = `{"msg":"TestBrokerTracing event-1"}`
	)
	return func(
		ctx context.Context,
		t *testing.T,
		channel *metav1.TypeMeta,
		client *testlib.Client,
		loggerPodName string,
		senderPublishTrace bool,
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
		client.CreatePodOrFail(logPod, testlib.WithService(loggerPodName))

		// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
		loggerTrigger := client.CreateTriggerOrFailV1Beta1(
			"logger",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etLogger, map[string]interface{}{}),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(loggerPodName),
		)

		// Create a transformer (EventTransfrmer) Pod that replies with the same event as the input,
		// except the reply's event's type is changed to bar.
		eventTransformerPod := resources.EventTransformationPod(
			"transformer",
			etLogger,
			senderName,
			[]byte(eventBody),
		)
		client.CreatePodOrFail(eventTransformerPod, testlib.WithService(eventTransformerPod.Name))

		// Create a Trigger that receives events (type=foo) and sends them to the transformer Pod.
		transformerTrigger := client.CreateTriggerOrFailV1Beta1(
			"transformer",
			resources.WithBrokerV1Beta1(broker.Name),
			resources.WithAttributesTriggerFilterV1Beta1(v1beta1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
			resources.WithSubscriberServiceRefForTriggerV1Beta1(eventTransformerPod.Name),
		)

		// Wait for all test resources to be ready, so that we can start sending events.
		client.WaitForAllTestResourcesReadyOrFail(ctx)

		// Everything is setup to receive an event. Generate a CloudEvent.
		event := cloudevents.NewEvent()
		event.SetID(eventID)
		event.SetSource(senderName)
		event.SetType(etTransformer)
		if err := event.SetData(cloudevents.ApplicationJSON, []byte(eventBody)); err != nil {
			t.Fatalf("Cannot set the payload of the event: %s", err.Error())
		}

		// Send the CloudEvent (either with or without tracing inside the SendEvents Pod).
		if senderPublishTrace {
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event, sender.EnableTracing())
		} else {
			client.SendEventToAddressable(ctx, senderName, broker.Name, testlib.BrokerTypeMeta, event)
		}

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

		if senderPublishTrace {
			expected = tracinghelper.TestSpanTree{
				Note: "1. Send pod sends event to the Broker Ingress (only if the sending pod generates a span).",
				Span: tracinghelper.MatchHTTPSpanNoReply(
					model.Client,
					tracinghelper.WithLocalEndpointServiceName(senderName),
				),
				Children: []tracinghelper.TestSpanTree{expected},
			}
		}

		return expected, cetest.AllOf(
			cetest.HasSource(senderName),
			cetest.HasId(eventID),
			cetest.DataContains(eventBody),
		)
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
