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
	"regexp"
	"strconv"

	cloudevents "github.com/cloudevents/sdk-go"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/trace"
)

const (
	// traceparentAttribute is the name of the CloudEvents attribute that contains the trace state.
	// See
	// https://github.com/cloudevents/spec/blob/v1.0-rc1/extensions/distributed-tracing.md#traceparent
	traceparentAttribute = "traceparent"
)

var (
	traceparent_regexp = regexp.MustCompile("^00-([a-f0-9]{32})-([a-f0-9]{16})-([a-f0-9]{2})$")
)

// AddTraceparentAttributeFromContext returns a CloudEvent that is identical to the input event,
// with the traceparent CloudEvents extension attribute added. The value for that attribute is the
// Span stored in the context.
//
// The context is expected to have passed through the OpenCensus HTTP Handler, so that the Span has
// been added to it.
func AddTraceparentAttributeFromContext(ctx context.Context, event cloudevents.Event) cloudevents.Event {
	span := trace.FromContext(ctx)
	if span != nil {
		event.SetExtension(traceparentAttribute, traceparentAttributeValue(span))
	}
	return event
}

func traceparentAttributeValue(span *trace.Span) string {
	flags := "00"
	if span.SpanContext().IsSampled() {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s",
		span.SpanContext().TraceID.String(),
		span.SpanContext().SpanID.String(),
		flags)
}

// AddSpanFromTraceparentAttribute extracts the traceparent extension attribute from the CloudEvent
// and returns a context with that span set. If the traceparent extension attribute is not found or
// cannot be parsed, then an error is returned.
func AddSpanFromTraceparentAttribute(ctx context.Context, name string, event cloudevents.Event) (context.Context, error) {
	tp, ok := event.Extensions()[traceparentAttribute]
	if !ok {
		return ctx, fmt.Errorf("extension attributes did not contain %q", traceparentAttribute)
	}
	tps, ok := tp.(string)
	if !ok {
		return ctx, fmt.Errorf("extension attribute %q's value was not a string: %T", traceparentAttribute, tps)
	}
	sc, err := parseTraceparent(tps)
	if err != nil {
		return ctx, err
	}
	// Create a fake Span with the saved information. In order to ensure any requests made with this
	// context have a parent of the saved Span, set the SpanID to the one saved in the traceparent.
	// Normally, a new SpanID is generated and because this Span is never reported would create a
	// hole in the Span tree.
	_, span := trace.StartSpanWithRemoteParent(ctx, name, sc)
	span.SetSpanID(sc.SpanID)
	return trace.NewContext(ctx, span), nil
}

func parseTraceparent(tp string) (trace.SpanContext, error) {
	m := traceparent_regexp.FindStringSubmatch(tp)
	if len(m) == 0 {
		return trace.SpanContext{}, fmt.Errorf("could not parse traceparent: %q", tp)
	}
	traceID, ok := b3.ParseTraceID(m[1])
	if !ok {
		return trace.SpanContext{}, fmt.Errorf("could not parse traceID: %q", tp)
	}
	spanID, ok := b3.ParseSpanID(m[2])
	if !ok {
		return trace.SpanContext{}, fmt.Errorf("could not parse spanID: %q", tp)
	}
	options, err := strconv.ParseUint(m[3], 16, 32)
	if err != nil {
		return trace.SpanContext{}, fmt.Errorf("could not parse options: %q", tp)
	}
	return trace.SpanContext{
		TraceID:      traceID,
		SpanID:       spanID,
		TraceOptions: trace.TraceOptions(options),
	}, nil
}
