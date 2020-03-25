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

	ce "github.com/cloudevents/sdk-go/v1"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	"knative.dev/eventing/pkg/apis/eventing"
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
	) (tracinghelper.TestSpanTree, lib.EventMatchFunc) {
		// Create the Broker.
		if brokerClass == eventing.ChannelBrokerClassValue {
			// create required RBAC resources including ServiceAccounts and ClusterRoleBindings for Brokers
			client.CreateConfigMapPropagationOrFail(defaultCMPName)
			client.CreateRBACResourcesForBrokers()
		}
		broker := client.CreateBrokerOrFail(
			"br",
			resources.WithBrokerClassForBroker(brokerClass),
			resources.WithChannelTemplateForBroker(channel),
		)

		// Create a logger (EventRecord) Pod and a K8s Service that points to it.
		logPod := resources.EventRecordPod(loggerPodName)
		client.CreatePodOrFail(logPod, lib.WithService(loggerPodName))

		// Create a Trigger that receives events (type=bar) and sends them to the logger Pod.
		client.CreateTriggerOrFail(
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
		client.CreateTriggerOrFail(
			"transformer",
			resources.WithBroker(broker.Name),
			resources.WithAttributesTriggerFilter(v1alpha1.TriggerAnyFilter, etTransformer, map[string]interface{}{}),
			resources.WithSubscriberServiceRefForTrigger(eventTransformerPod.Name),
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

		// TODO Actually determine the cluster's domain, similar to knative.dev/pkg/network/domain.go.
		domain := "cluster.local"

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

		// Steps 7-10: Event from TrChannel sent to transformer Trigger and its reply to the InChannel.
		transformerEventSentFromTrChannelToTransformer := tracinghelper.TestSpanTree{
			Note: "3. Broker Filter for the 'transformer' trigger sends the event to the transformer pod.",
			Span: tracinghelper.MatchHTTPSpanWithReply(
				model.Client,
				tracinghelper.WithHTTPHostAndPath(transformerSVCHost, "/"),
			),
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

		// Step 11-20. Directly steps 11-12. Steps 13-20 are children.
		// Steps 11-12 Reply from the 'transformer' is sent by the Broker TrChannel to the Broker
		// Ingress.
		transformerEventResponseFromTrChannel := tracinghelper.TestSpanTree{
			Note: "5. Broker Filter for the 'logger' trigger sends the event to the logger pod.",
			Span: tracinghelper.MatchHTTPSpanNoReply(
				model.Client,
				tracinghelper.WithHTTPHostAndPath(loggerSVCHost, "/"),
			),
			Children: []tracinghelper.TestSpanTree{
				{
					Note: "6. Logger pod receives the event from the Broker Filter for the 'logger' trigger.",
					Span: tracinghelper.MatchHTTPSpanNoReply(
						model.Server,
						tracinghelper.WithHTTPHostAndPath(loggerSVCHost, "/"),
						tracinghelper.WithLocalEndpointServiceName(loggerPodName),
					),
				},
			},
		}

		// Steps 0-22. Directly steps 0-4 (missing 1).
		// Steps 0-4 (missing 1, which is optional and added below if present): Event sent to the Broker
		// Ingress.
		expected := tracinghelper.TestSpanTree{
			Note: "2. Broker Ingress receives the event from the sending pod.",
			Span: tracinghelper.MatchHTTPSpanNoReply(model.Server),
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
		matchFunc := func(ev ce.Event) bool {
			if ev.Source() != senderName {
				return false
			}
			if ev.ID() != eventID {
				return false
			}
			db, _ := ev.DataBytes()
			return strings.Contains(string(db), body)
		}

		return expected, matchFunc
	}
}
