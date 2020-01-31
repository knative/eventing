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

package tracing

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.opencensus.io/trace"
)

func TestAddTraceparentAttributeFromContext(t *testing.T) {
	testCases := map[string]struct {
		inCTX   bool
		sampled bool
	}{
		"no span in context": {},
		"not sampled": {
			inCTX: true,
		},
		"sampled": {
			inCTX:   true,
			sampled: true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			sampler := trace.WithSampler(trace.NeverSample())
			if tc.sampled {
				sampler = trace.WithSampler(trace.AlwaysSample())
			}
			_, span := trace.StartSpan(context.Background(), "name", sampler)

			ctx := context.Background()
			if tc.inCTX {
				ctx = trace.NewContext(ctx, span)
			}
			event := cloudevents.Event{
				Context: &cloudevents.EventContextV03{
					ID: "from-the-test",
				},
			}
			eventWithTraceparent := AddTraceparentAttributeFromContext(ctx, event)
			if tc.inCTX {
				sampled := "00"
				if tc.sampled {
					sampled = "01"
				}
				tp, ok := eventWithTraceparent.Extensions()["traceparent"]
				if !ok {
					t.Fatal("traceparent annotation not present.")
				}
				expected := fmt.Sprintf("00-%s-%s-%s", span.SpanContext().TraceID, span.SpanContext().SpanID, sampled)
				if expected != tp.(string) {
					t.Fatalf("Unexpected traceparent value. Got %q. Want %q", tp.(string), expected)
				}
			} else {
				if diff := cmp.Diff(event, eventWithTraceparent); diff != "" {
					t.Fatalf("%s: Event changed unexpectedly (-want, +got) = %v", n, diff)
				}
			}
		})
	}
}

func TestAddSpanFromTraceparentAttribute(t *testing.T) {
	traceID := "1234567890abcdef1234567890abcdef"
	spanID := "1234567890abcdef"
	sampledOptions := "01"
	notSampledOptions := "00"
	testCases := map[string]struct {
		present     bool
		notAString  bool
		sampled     bool
		traceparent string
		expectError bool
	}{
		"not present": {
			expectError: true,
		},
		"not a string": {
			present:     true,
			notAString:  true,
			expectError: true,
		},
		"bad format": {
			present:     true,
			traceparent: "random-string",
			expectError: true,
		},
		"bad traceID": {
			present:     true,
			traceparent: fmt.Sprintf("00-%s-%s-%s", "bad", spanID, sampledOptions),
			expectError: true,
		},
		"bad spanID": {
			present:     true,
			traceparent: fmt.Sprintf("00-%s-%s-%s", traceID, "bad", sampledOptions),
			expectError: true,
		},
		"bad options": {
			present:     true,
			traceparent: fmt.Sprintf("00-%s-%s-%s", traceID, spanID, "bad"),
			expectError: true,
		},
		"good": {
			present:     true,
			sampled:     true,
			traceparent: fmt.Sprintf("00-%s-%s-%s", traceID, spanID, sampledOptions),
		},
		"not sampled": {
			present:     true,
			sampled:     false,
			traceparent: fmt.Sprintf("00-%s-%s-%s", traceID, spanID, notSampledOptions),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			event := cloudevents.Event{
				Context: &cloudevents.EventContextV03{},
			}
			if tc.present {
				if tc.notAString {
					event.SetExtension("traceparent", 1.0)
				} else {
					event.SetExtension("traceparent", tc.traceparent)
				}
			}
			ctx, err := AddSpanFromTraceparentAttribute(context.Background(), "name", event)
			if tc.expectError {
				if err == nil {
					t.Fatal("Expected an error, actually nil")
				}
				return
			}
			span := trace.FromContext(ctx)
			if actual := span.SpanContext().TraceID.String(); traceID != actual {
				t.Errorf("Incorrect TraceID. Got %q. Want %q", actual, traceID)
			}
			if actual := span.SpanContext().SpanID.String(); spanID != actual {
				t.Errorf("Incorrect SpanID. Got %q. Want %q", actual, spanID)
			}

			wantOptions := uint32(0)
			if tc.sampled {
				wantOptions = uint32(1)
			}
			if actualOptions := uint32(span.SpanContext().TraceOptions); wantOptions != actualOptions {
				t.Errorf("Incorrect options. Got %q. Want %q", actualOptions, wantOptions)
			}
		})
	}
}
