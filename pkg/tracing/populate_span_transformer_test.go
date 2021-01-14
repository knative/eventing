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
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"
)

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
	wantEvent.SetType("hello.world")
	wantEvent.SetSource("example.com")

	destination := "some-url"

	expectedAttributes := map[string]interface{}{
		"messaging.system":        "knative",
		"messaging.protocol":      "HTTP",
		"messaging.message_id":    "aaa",
		"messaging.destination":   "some-url",
		"cloudevents.id":          "aaa",
		"cloudevents.type":        "hello.world",
		"cloudevents.source":      "example.com",
		"cloudevents.specversion": "1.0",
	}

	bindingtest.RunTransformerTests(t, context.Background(), []bindingtest.TransformerTestArgs{
		{
			Name:         "Populate span for binary messages",
			InputMessage: bindingtest.MustCreateMockBinaryMessage(wantEvent),
			AssertFunc: func(t *testing.T, haveEvent event.Event) {
				test.AssertEventEquals(t, wantEvent, haveEvent) // Event should be unchanged
				testSpanBinary.End()
				spanData := <-mockExp
				require.Equal(t, expectedAttributes, spanData.Attributes)
			},
			Transformers: binding.Transformers{PopulateSpan(testSpanBinary, destination)},
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
			Transformers: binding.Transformers{PopulateSpan(testSpanEvent, destination)},
		},
	})
}
