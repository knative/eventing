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
	"net/http"
	"testing"
	"time"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/openzipkin/zipkin-go/model"
	"go.opentelemetry.io/otel/api/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/zipkin"

	tracinghelper "knative.dev/eventing/test/conformance/helpers/tracing"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
)

// SetupTracingTestInfrastructureFunc sets up the infrastructure for running tracing tests. It returns the
// expected trace as well as a string that is expected to be in the logger Pod's logs.
type SetupTracingTestInfrastructureFunc func(
	t *testing.T,
	channel *metav1.TypeMeta,
	client *testlib.Client,
	loggerPodName string,
	senderPublishTrace bool,
) (tracinghelper.TestSpanTree, cetest.EventMatcher)

// tracingTest bootstraps the test and then executes the assertions on the received event and on the spans
func tracingTest(
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

	// Do NOT call zipkin.CleanupZipkinTracingSetup. That will be called exactly once in
	// TestMain.
	tracinghelper.Setup(t, client)

	// Setup the test infrastructure
	expectedTestSpan, eventMatcher := setupInfrastructure(t, &channel, client, recordEventsPodName, true)

	// Start the event info store and assert the event was received correctly
	targetTracker, err := recordevents.NewEventInfoStore(client, recordEventsPodName)
	if err != nil {
		t.Fatalf("Pod tracker failed: %v", err)
	}
	matches := targetTracker.AssertAtLeast(1, recordevents.MatchEvent(eventMatcher))

	// Match the trace
	traceID := getTraceIDHeader(t, matches)
	trace, err := zipkin.JSONTracePred(traceID, 5*time.Minute, func(trace []model.SpanModel) bool {
		tree, err := tracinghelper.GetTraceTree(trace)
		if err != nil {
			return false
		}
		// Do not pass t to prevent unnecessary log output.
		return len(expectedTestSpan.MatchesSubtree(nil, tree)) > 0
	})
	if err != nil {
		t.Logf("Unable to get trace %q: %v. Trace so far %+v", traceID, err, tracinghelper.PrettyPrintTrace(trace))
		tree, err := tracinghelper.GetTraceTree(trace)
		if err != nil {
			t.Fatal(err)
		}
		if len(expectedTestSpan.MatchesSubtree(t, tree)) == 0 {
			t.Fatalf("No matching subtree. want: %v got: %v", expectedTestSpan, tree)
		}
	}
}

// getTraceIDHeader gets the TraceID from the passed in events.  It returns the header from the
// first matching event, but registers a fatal error if more than one traceid header is seen
// in that message.
func getTraceIDHeader(t *testing.T, evInfos []recordevents.EventInfo) string {
	for i := range evInfos {
		if nil != evInfos[i].HTTPHeaders {
			sc := trace.RemoteSpanContextFromContext(trace.DefaultHTTPPropagator().Extract(context.TODO(), http.Header(evInfos[i].HTTPHeaders)))
			if sc.HasTraceID() {
				return sc.TraceIDString()
			}
		}
	}
	t.Fatalf("FAIL: No traceid in %d messages: (%s)", len(evInfos), evInfos)
	return ""
}
