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

package tracing

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
)

const (
	traceparentAttribute = "Traceparent"
)

func traceparentAttributeValue(span *trace.Span) string {
	flags := "00"
	if span.SpanContext().IsSampled() {
		flags = "01"
	}
	return "00-" + span.SpanContext().TraceID.String() + "-" +
		span.SpanContext().SpanID.String() + "-" + flags
}

func TestTraceparentTransformer(t *testing.T) {
	_, span := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.NeverSample()))
	_, newSpan := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.NeverSample()))

	withoutTraceparent := test.MinEvent()
	withoutTraceparentExpected := withoutTraceparent.Clone()
	withoutTraceparentExpected.SetExtension(traceparentAttribute, traceparentAttributeValue(span))

	withTraceparent := test.MinEvent()
	withTraceparent.SetExtension(traceparentAttribute, traceparentAttributeValue(span))
	withTraceparentExpected := withTraceparent.Clone()
	withTraceparentExpected.SetExtension(traceparentAttribute, traceparentAttributeValue(newSpan))

	test.RunTransformerTests(t, context.Background(), []test.TransformerTestArgs{
		{
			Name:         "Add traceparent in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withoutTraceparent),
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Add traceparent in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withoutTraceparent),
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Add traceparent in Event message",
			InputEvent:   withoutTraceparent,
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Update traceparent in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withTraceparent),
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
		{
			Name:         "Update traceparent in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withTraceparent),
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
		{
			Name:         "Update traceparent in Event message",
			InputEvent:   withTraceparent,
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
	})
}

type mockExporter chan *trace.SpanData

func (m mockExporter) ExportSpan(s *trace.SpanData) {
	m <- s
}

func TestPopulateSpan(t *testing.T) {
	mockExp := make(mockExporter, 1)
	trace.RegisterExporter(mockExp)

	_, testSpanBinary := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.AlwaysSample()))
	_, testSpanEvent := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.AlwaysSample()))

	wantEvent := event.New(event.CloudEventsVersionV1)
	wantEvent.SetID("aaa")
	wantEvent.SetDataContentType("application/json")
	wantEvent.SetSubject("sub")
	wantEvent.SetType("hello.world")
	wantEvent.SetSource("example.com")

	expectedAttributes := map[string]interface{}{
		"cloudevents.id":              "aaa",
		"cloudevents.datacontenttype": "application/json",
		"cloudevents.subject":         "sub",
		"cloudevents.type":            "hello.world",
		"cloudevents.source":          "example.com",
		"cloudevents.specversion":     "1.0",
	}

	test.RunTransformerTests(t, context.Background(), []test.TransformerTestArgs{
		{
			Name:         "Populate span for binary messages",
			InputMessage: test.MustCreateMockBinaryMessage(wantEvent),
			AssertFunc: func(t *testing.T, haveEvent event.Event) {
				test.AssertEventEquals(t, wantEvent, haveEvent) // Event should be unchanged
				testSpanBinary.End()
				spanData := <-mockExp
				require.Equal(t, expectedAttributes, spanData.Attributes)
			},
			Transformers: []binding.TransformerFactory{PopulateSpan(testSpanBinary)},
		},
		{
			Name:       "Populate span for event messages",
			InputEvent: wantEvent,
			AssertFunc: func(t *testing.T, haveEvent event.Event) {
				test.AssertEventEquals(t, wantEvent, haveEvent) // Event should be unchanged
				testSpanEvent.End()
				spanData := <-mockExp
				require.Equal(t, expectedAttributes, spanData.Attributes)
			},
			Transformers: []binding.TransformerFactory{PopulateSpan(testSpanEvent)},
		},
	})
}
