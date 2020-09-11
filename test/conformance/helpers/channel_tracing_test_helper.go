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
	"github.com/google/uuid"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/eventing/test/lib/sender"
)

// ChannelTracingTestHelperWithChannelTestRunner runs the Channel tracing tests for all Channels in
// the ComponentsTestRunner.
func ChannelTracingTestHelperWithChannelTestRunner(
	ctx context.Context,
	t *testing.T,
	channelTestRunner testlib.ComponentsTestRunner,
	setupClient testlib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, testlib.FeatureBasic, func(t *testing.T, channel metav1.TypeMeta) {
		tracingTest(ctx, t, setupClient, setupChannelTracingWithReply, channel)
	})
}

// setupChannelTracing is the general setup for TestChannelTracing. It creates the following:
// SendEvents (Pod) -> Channel -> Subscription -> K8s Service -> Mutate (Pod)
//                                                                   v
// LogEvents (Pod) <- K8s Service <- Subscription  <- Channel <- (Reply) Subscription
// It returns the expected trace tree and a match function that is expected to be sent
// by the SendEvents Pod and should be present in the RecordEvents list of events.
func setupChannelTracingWithReply(
	ctx context.Context,
	t *testing.T,
	channel *metav1.TypeMeta,
	client *testlib.Client,
	recordEventsPodName string,
	senderPublishTrace bool,
) (tracinghelper.TestSpanTree, cetest.EventMatcher) {
	eventSource := "sender"
	// Create the Channels.
	channelName := "ch"
	client.CreateChannelOrFail(channelName, channel)

	replyChannelName := "reply-ch"
	client.CreateChannelOrFail(replyChannelName, channel)

	// Create the 'sink', a LogEvents Pod and a K8s Service that points to it.
	recordEventsPod := resources.EventRecordPod(recordEventsPodName)
	client.CreatePodOrFail(recordEventsPod, testlib.WithService(recordEventsPodName))

	// Create the subscriber, a Pod that mutates the event.
	transformerPod := resources.EventTransformationPod(
		"transformer",
		"mutated",
		eventSource,
		nil,
	)
	client.CreatePodOrFail(transformerPod, testlib.WithService(transformerPod.Name))

	// Create the Subscription linking the Channel to the mutator.
	client.CreateSubscriptionOrFail(
		"sub",
		channelName,
		channel,
		resources.WithSubscriberForSubscription(transformerPod.Name),
		resources.WithReplyForSubscription(replyChannelName, channel))

	// Create the Subscription linking the reply Channel to the LogEvents K8s Service.
	client.CreateSubscriptionOrFail(
		"reply-sub",
		replyChannelName,
		channel,
		resources.WithSubscriberForSubscription(recordEventsPodName),
	)

	// Wait for all test resources to be ready, so that we can start sending events.
	client.WaitForAllTestResourcesReadyOrFail(ctx)

	// Everything is setup to receive an event. Generate a CloudEvent.
	senderName := "sender"
	eventID := uuid.New().String()
	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetSource(senderName)
	event.SetType(testlib.DefaultEventType)
	body := fmt.Sprintf(`{"msg":"TestChannelTracing %s"}`, eventID)
	if err := event.SetData(cloudevents.ApplicationJSON, []byte(body)); err != nil {
		t.Fatalf("Cannot set the payload of the event: %s", err.Error())
	}

	// Send the CloudEvent (either with or without tracing inside the SendEvents Pod).
	if senderPublishTrace {
		client.SendEventToAddressable(ctx, senderName, channelName, channel, event, sender.EnableTracing())
	} else {
		client.SendEventToAddressable(ctx, senderName, channelName, channel, event)
	}

	// We expect the following spans:
	// 1. Sending pod sends event to Channel (only if the sending pod generates a span).
	// 2. Channel receives event from sending pod.
	// 3. Channel sends event to transformer pod.
	// 4. Transformer Pod receives event from Channel.
	// 5. Channel sends reply from Transformer Pod to the reply Channel.
	// 6. Reply Channel receives event from the original Channel's reply.
	// 7. Reply Channel sends event to the logging Pod.
	// 8. Logging pod receives event from Channel.
	expected := tracinghelper.TestSpanTree{
		// 1 is added below if it is needed.
		// 2. Channel receives event from sending pod.
		Span: tracinghelper.MatchHTTPSpanNoReply(
			model.Server,
			tracinghelper.WithHTTPHostAndPath(
				fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", channelName, client.Namespace),
				"/",
			),
		),
		Children: []tracinghelper.TestSpanTree{
			{
				// 3. Channel sends event to transformer pod.
				Span: tracinghelper.MatchHTTPSpanWithReply(
					model.Client,
					tracinghelper.WithHTTPHostAndPath(
						fmt.Sprintf("%s.%s.svc.cluster.local", transformerPod.Name, client.Namespace),
						"/",
					),
				),
				Children: []tracinghelper.TestSpanTree{
					{
						// 4. Transformer Pod receives event from Channel.
						Span: tracinghelper.MatchHTTPSpanWithReply(
							model.Server,
							tracinghelper.WithHTTPHostAndPath(
								fmt.Sprintf("%s.%s.svc.cluster.local", transformerPod.Name, client.Namespace),
								"/",
							),
							tracinghelper.WithLocalEndpointServiceName(transformerPod.Name),
						),
					},
				},
			},
			{
				// 5. Channel sends reply from Transformer Pod to the reply Channel.
				Span: tracinghelper.MatchHTTPSpanNoReply(
					model.Client,
					tracinghelper.WithHTTPHostAndPath(
						fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", replyChannelName, client.Namespace),
						"",
					),
				),
				Children: []tracinghelper.TestSpanTree{
					// 6. Reply Channel receives event from the original Channel's reply.
					{
						Span: tracinghelper.MatchHTTPSpanNoReply(
							model.Server,
							tracinghelper.WithHTTPHostAndPath(
								fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", replyChannelName, client.Namespace),
								"/",
							),
						),
						Children: []tracinghelper.TestSpanTree{
							{
								// 7. Reply Channel sends event to the logging Pod.
								Span: tracinghelper.MatchHTTPSpanNoReply(
									model.Client,
									tracinghelper.WithHTTPHostAndPath(
										fmt.Sprintf("%s.%s.svc.cluster.local", recordEventsPod.Name, client.Namespace),
										"/",
									),
								),
								Children: []tracinghelper.TestSpanTree{
									{
										// 8. Logging pod receives event from Channel.
										Span: tracinghelper.MatchHTTPSpanNoReply(
											model.Server,
											tracinghelper.WithHTTPHostAndPath(
												fmt.Sprintf("%s.%s.svc.cluster.local", recordEventsPod.Name, client.Namespace),
												"/",
											),
											tracinghelper.WithLocalEndpointServiceName(recordEventsPod.Name),
										),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if senderPublishTrace {
		expected = tracinghelper.TestSpanTree{
			// 1. Sending pod sends event to Channel (only if the sending pod generates a span).
			Span: tracinghelper.MatchHTTPSpanNoReply(
				model.Client,
				tracinghelper.WithHTTPHostAndPath(
					fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", channelName, client.Namespace),
					"",
				),
				tracinghelper.WithLocalEndpointServiceName("sender"),
			),
			Children: []tracinghelper.TestSpanTree{expected},
		}
	}

	return expected, cetest.AllOf(
		cetest.HasSource(senderName),
		cetest.HasId(eventID),
		cetest.DataContains(body),
	)
}
