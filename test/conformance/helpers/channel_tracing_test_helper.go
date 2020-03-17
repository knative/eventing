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
	"time"

	ce "github.com/cloudevents/sdk-go/legacy"
	"github.com/openzipkin/zipkin-go/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"knative.dev/pkg/test/zipkin"

	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

// SetupInfrastructureFunc sets up the infrastructure for running tracing tests. It returns the
// expected trace as well as a string that is expected to be in the logger Pod's logs.
type SetupInfrastructureFunc func(
	t *testing.T,
	channel *metav1.TypeMeta,
	client *lib.Client,
	loggerPodName string,
	tc TracingTestCase,
) (tracinghelper.TestSpanTree, lib.EventMatchFunc)

// TracingTestCase is the test case information for tracing tests.
type TracingTestCase struct {
	// IncomingTraceId controls whether the original request is sent to the Broker/Channel already
	// has a trace ID associated with it by the sender.
	IncomingTraceId bool
	// Istio controls whether the Pods being created for the test (sender, transformer, logger,
	// etc.) have Istio sidecars. It does not affect the Channel Pods.
	Istio bool
}

// ChannelTracingTestHelperWithChannelTestRunner runs the Channel tracing tests for all Channels in
// the ChannelTestRunner.
func ChannelTracingTestHelperWithChannelTestRunner(
	t *testing.T,
	channelTestRunner lib.ChannelTestRunner,
	setupClient lib.SetupClientOption,
) {
	channelTestRunner.RunTests(t, lib.FeatureBasic, func(st *testing.T, channel metav1.TypeMeta) {
		// Don't accidentally use t, use st instead. To ensure this, shadow 't' to a useless type.
		t := struct{}{}
		_ = fmt.Sprintf("%s", t)

		ChannelTracingTestHelper(st, channel, setupClient)
	})
}

// ChannelTracingTestHelper runs the Channel tracing test using the given TypeMeta.
func ChannelTracingTestHelper(t *testing.T, channel metav1.TypeMeta, setupClient lib.SetupClientOption) {
	testCases := map[string]TracingTestCase{
		"includes incoming trace id": {
			IncomingTraceId: true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tracingTest(t, setupClient, setupChannelTracingWithReply, channel, tc)
		})
	}
}

func tracingTest(
	t *testing.T,
	setupClient lib.SetupClientOption,
	setupInfrastructure SetupInfrastructureFunc,
	channel metav1.TypeMeta,
	tc TracingTestCase,
) {
	const (
		loggerPodName = "logger"
	)

	client := lib.Setup(t, true, setupClient)
	defer lib.TearDown(client)

	// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
	// TestMain.
	tracinghelper.Setup(t, client)

	expected, mustMatch := setupInfrastructure(t, &channel, client, loggerPodName, tc)
	matches := assertEventMatch(t, client, loggerPodName, mustMatch)

	traceID := getTraceIDHeader(t, matches)
	trace, err := zipkin.JSONTracePred(traceID, 2*time.Minute, func(trace []model.SpanModel) bool {
		tree, err := tracinghelper.GetTraceTree(trace)
		if err != nil {
			return false
		}
		// Do not pass t to prevent unnecessary log output.
		return len(expected.MatchesSubtree(nil, tree)) > 0
	})
	if err != nil {
		t.Logf("Unable to get trace %q: %v. Trace so far %+v", traceID, err, tracinghelper.PrettyPrintTrace(trace))
		tree, err := tracinghelper.GetTraceTree(trace)
		if err != nil {
			t.Fatal(err)
		}
		if len(expected.MatchesSubtree(t, tree)) == 0 {
			t.Fatalf("No matching subtree. want: %v got: %v", expected, tree)
		}
	}
}

// assertEventMatch verifies that recorder pod contains at least one event that
// matches mustMatch. It is used to show that the expected event was sent to
// the logger Pod.  It returns a list of the matching events.
func assertEventMatch(t *testing.T, client *lib.Client, recorderPodName string,
	mustMatch lib.EventMatchFunc) []lib.EventInfo {
	targetTracker, err := client.NewEventInfoStore(recorderPodName, t.Logf)
	if err != nil {
		t.Fatalf("Pod tracker failed: %v", err)
	}
	defer targetTracker.Cleanup()
	matches, err := targetTracker.WaitAtLeastNMatch(lib.ValidEvFunc(mustMatch), 1)
	if err != nil {
		t.Fatalf("Expected messages not found: %v", err)
	}
	return matches
}

// getTraceIDHeader gets the TraceID from the passed in events.  It returns the header from the
// first matching event, but registers a fatal error if more than one traceid header is seen
// in that message.
func getTraceIDHeader(t *testing.T, evInfos []lib.EventInfo) string {
	for i := range evInfos {
		if nil != evInfos[i].HTTPHeaders {
			traceID, found := evInfos[i].HTTPHeaders["X-B3-Traceid"]
			if found {
				if len(traceID) != 1 {
					t.Fatalf("Unexpected length %d for traceid list: %v",
						len(traceID), traceID)
				}
				return strings.TrimSpace(traceID[0])
			}
		}
	}
	t.Fatalf("FAIL: No traceid in %d messages: (%s)", len(evInfos), evInfos)
	return ""
}

// setupChannelTracing is the general setup for TestChannelTracing. It creates the following:
// SendEvents (Pod) -> Channel -> Subscription -> K8s Service -> Mutate (Pod)
//                                                                   v
// LogEvents (Pod) <- K8s Service <- Subscription  <- Channel <- (Reply) Subscription
// It returns the expected trace tree and a match function that is expected to be sent
// by the SendEvents Pod and should be present in the RecordEvents list of events.
func setupChannelTracingWithReply(
	t *testing.T,
	channel *metav1.TypeMeta,
	client *lib.Client,
	loggerPodName string,
	tc TracingTestCase,
) (tracinghelper.TestSpanTree, lib.EventMatchFunc) {
	// Create the Channels.
	channelName := "ch"
	client.CreateChannelOrFail(channelName, channel)

	replyChannelName := "reply-ch"
	client.CreateChannelOrFail(replyChannelName, channel)

	// Create the 'sink', a LogEvents Pod and a K8s Service that points to it.
	loggerPod := resources.EventRecordPod(loggerPodName)
	client.CreatePodOrFail(loggerPod, lib.WithService(loggerPodName))

	// Create the subscriber, a Pod that mutates the event.
	transformerPod := resources.EventTransformationPod("transformer", &cloudevents.CloudEvent{
		EventContextV1: ce.EventContextV1{
			Type: "mutated",
		},
	})
	client.CreatePodOrFail(transformerPod, lib.WithService(transformerPod.Name))

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
		resources.WithSubscriberForSubscription(loggerPodName),
	)

	// Wait for all test resources to be ready, so that we can start sending events.
	client.WaitForAllTestResourcesReadyOrFail()

	// Everything is setup to receive an event. Generate a CloudEvent.
	senderName := "sender"
	eventID := string(uuid.NewUUID())
	body := fmt.Sprintf("TestChannelTracing %s", eventID)
	event := cloudevents.New(
		fmt.Sprintf(`{"msg":%q}`, body),
		cloudevents.WithSource(senderName),
		cloudevents.WithID(eventID),
	)

	// Send the CloudEvent (either with or without tracing inside the SendEvents Pod).
	sendEvent := client.SendFakeEventToAddressableOrFail
	if tc.IncomingTraceId {
		sendEvent = client.SendFakeEventWithTracingToAddressableOrFail
	}
	sendEvent(senderName, channelName, channel, event)

	// We expect the following spans:
	// 0. Artificial root span.
	// 1. Sending pod sends event to Channel (only if the sending pod generates a span).
	// 2. Channel receives event from sending pod.
	// 3. Channel sends event to transformer pod.
	// 4. Transformer Pod receives event from Channel.
	// 5. Channel sends reply from Transformer Pod to the reply Channel.
	// 6. Reply Channel receives event from the original Channel's reply.
	// 7. Reply Channel sends event to the logging Pod.
	// 8. Logging pod receives event from Channel.
	expected := tracinghelper.TestSpanTree{
		// 0. Artificial root span.
		Root: true,
		// 1 is added below if it is needed.
		Children: []tracinghelper.TestSpanTree{
			{
				// 2. Channel receives event from sending pod.
				Span: tracinghelper.MatchHTTPServerSpanNoReply(fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", channelName, client.Namespace), "/"),
				Children: []tracinghelper.TestSpanTree{
					{
						// 3. Channel sends event to transformer pod.
						Span: tracinghelper.MatchHTTPClientSpanWithReply(
							fmt.Sprintf("%s.%s.svc.cluster.local", transformerPod.Name, client.Namespace), "/"),
						Children: []tracinghelper.TestSpanTree{
							{
								// 4. Transformer Pod receives event from Channel.
								Span: tracinghelper.MatchHTTPServerSpanWithReply(
									fmt.Sprintf("%s.%s.svc.cluster.local", transformerPod.Name, client.Namespace),
									"/",
									tracinghelper.WithLocalEndpointServiceName(transformerPod.Name),
								),
							},
						},
					},
					{
						// 5. Channel sends reply from Transformer Pod to the reply Channel.
						Span: tracinghelper.MatchHTTPClientSpanNoReply(
							fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", replyChannelName, client.Namespace), ""),
						Children: []tracinghelper.TestSpanTree{
							// 6. Reply Channel receives event from the original Channel's reply.
							{
								Span: tracinghelper.MatchHTTPServerSpanNoReply(
									fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", replyChannelName, client.Namespace), "/"),
								Children: []tracinghelper.TestSpanTree{
									{
										// 7. Reply Channel sends event to the logging Pod.
										Span: tracinghelper.MatchHTTPClientSpanNoReply(
											fmt.Sprintf("%s.%s.svc.cluster.local", loggerPod.Name, client.Namespace), "/"),
										Children: []tracinghelper.TestSpanTree{
											{
												// 8. Logging pod receives event from Channel.
												Span: tracinghelper.MatchHTTPServerSpanNoReply(
													fmt.Sprintf("%s.%s.svc.cluster.local", loggerPod.Name, client.Namespace),
													"/",
													tracinghelper.WithLocalEndpointServiceName(loggerPod.Name),
												),
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

	if tc.IncomingTraceId {
		expected.Children = []tracinghelper.TestSpanTree{
			{
				// 1. Sending pod sends event to Channel (only if the sending pod generates a span).
				Span: tracinghelper.MatchHTTPClientSpanNoReply(
					fmt.Sprintf("%s-kn-channel.%s.svc.cluster.local", channelName, client.Namespace),
					"",
					tracinghelper.WithLocalEndpointServiceName("sender"),
				),
				Children: expected.Children,
			},
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
