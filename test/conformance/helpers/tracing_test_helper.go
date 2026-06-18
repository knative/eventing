/*
Copyright 2020 The Knative Authors

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
	"testing"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
)

// SetupTracingTestInfrastructureFunc sets up the infrastructure for running tracing tests. It returns the
// expected trace as well as a string that is expected to be in the logger Pod's logs.
type SetupTracingTestInfrastructureFunc func(
	ctx context.Context,
	t *testing.T,
	channel *metav1.TypeMeta,
	client *testlib.Client,
	loggerPodName string,
	senderPublishTrace bool,
) (tracinghelper.TestSpanTree, cetest.EventMatcher)

// tracingTest bootstraps the test and then executes the assertions on the received event and on the spans
func tracingTest(
	ctx context.Context,
	t *testing.T,
	setupClient testlib.SetupClientOption,
	setupInfrastructure SetupTracingTestInfrastructureFunc,
	channel metav1.TypeMeta,
) {
	const (
		recordEventsPodName = "recordevents"
	)

	client := testlib.Setup(t, true, setupClient)
	defer testlib.TearDown(client)

	// Start the event info store. Note this is done _before_ we setup the infrastructure, which
	// sends the event.
	targetTracker, err := recordevents.NewEventInfoStore(client, recordEventsPodName, client.Namespace)
	if err != nil {
		t.Fatal("Pod tracker failed:", err)
	}

	// Setup the test infrastructure
	expectedTestSpan, eventMatcher := setupInfrastructure(ctx, t, &channel, client, recordEventsPodName, true)

	// Assert that the event was seen.
	matches := targetTracker.AssertAtLeast(1, recordevents.MatchEvent(eventMatcher))

	// Match the trace
	traceID := getTraceIDHeader(t, matches)

	// TODO(knative/eventing#8853): Once the OTel collector is deployed in test infrastructure,
	// query the collector here to retrieve spans for the given traceID, build a SpanTree, and
	// match against expectedTestSpan. For now, log the expected tree and trace ID.
	t.Logf("Trace ID: %s", traceID)
	t.Logf("Expected span tree: %s", expectedTestSpan)

	// Give time for spans to be exported.
	time.Sleep(10 * time.Second)

	// TODO(knative/eventing#8853): Replace the following with actual OTel trace retrieval and matching:
	// spans, err := fetchOTelTraceByID(traceID, 5*time.Minute)
	// if err != nil {
	// 	t.Fatalf("Unable to get trace %q: %v", traceID, err)
	// }
	// tree, err := tracinghelper.GetTraceTree(spans)
	// if err != nil {
	//	t.Fatal(err)
	// }
	// if len(expectedTestSpan.MatchesSubtree(t, tree)) == 0 {
	// 	t.Fatalf("No matching subtree. want: %v got: %v", expectedTestSpan, tree)
	// }
}

// getTraceIDHeader gets the TraceID from the passed in events.  It returns the header from the
// first matching event, but registers a fatal error if more than one traceid header is seen
// in that message.
func getTraceIDHeader(t *testing.T, evInfos []recordevents.EventInfo) string {
	for i := range evInfos {
		if nil != evInfos[i].HTTPHeaders {
			sc := trace.SpanContextFromContext(propagation.TraceContext{}.Extract(context.TODO(), propagation.HeaderCarrier(evInfos[i].HTTPHeaders)))
			if sc.HasTraceID() {
				return sc.TraceID().String()
			}
		}
	}
	t.Fatalf("FAIL: No traceid in %d messages: (%v)", len(evInfos), evInfos)
	return ""
}
