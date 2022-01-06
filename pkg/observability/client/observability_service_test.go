/*
Copyright 2022 The Knative Authors

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

package client

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/trace"

	"knative.dev/eventing/pkg/observability"
)

type mockExporter chan *trace.SpanData

func (m mockExporter) ExportSpan(s *trace.SpanData) {
	m <- s
}

func TestKnativeObservabilityServiceRequestSend(t *testing.T) {
	mockExp := make(mockExporter, 1)
	trace.RegisterExporter(mockExp)

	trace.ApplyConfig(trace.Config{
		DefaultSampler: trace.AlwaysSample(),
	})

	event := event.New(event.CloudEventsVersionV1)
	event.SetID("aaa")
	event.SetType("hello.world")
	event.SetSource("example.com")

	ctx := context.Background()
	ctx = observability.WithSpanData(ctx, "spanname", 1, []trace.Attribute{trace.StringAttribute("myattr", "myvalue")})

	_, callback := New().RecordSendingEvent(ctx, event)
	callback(nil)

	trace := <-mockExp

	expectedAttributes := map[string]interface{}{
		"cloudevents.id":          "aaa",
		"cloudevents.type":        "hello.world",
		"cloudevents.source":      "example.com",
		"cloudevents.specversion": "1.0",
		"myattr":                  "myvalue",
	}
	require.Equal(t, expectedAttributes, trace.Attributes)
}
