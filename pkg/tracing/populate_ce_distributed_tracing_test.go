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
	"net/http"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"knative.dev/pkg/observability/tracing"
)

func TestPopulateCEDistributedTracing(t *testing.T) {
	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	exporter := tracetest.NewInMemoryExporter()
	tp := trace.NewTracerProvider(trace.WithSyncer(exporter))

	origCtx := context.Background()

	tracer := tp.Tracer("ce-distributed-tracing-test")

	spanCtx, testSpan := tracer.Start(origCtx, "name")
	testSpan.SetAttributes(
		attribute.String("client", "A"),
	)

	wantEvent := event.New(event.CloudEventsVersionV1)
	wantEvent.SetID("aaa")
	wantEvent.SetType("hello.world")
	wantEvent.SetSource("example.com")

	bindingtest.RunTransformerTests(t, context.Background(), []bindingtest.TransformerTestArgs{
		{
			Name:         "Check propagation",
			InputMessage: bindingtest.MustCreateMockBinaryMessage(wantEvent),
			AssertFunc: func(t *testing.T, haveEvent event.Event) {
				require.Equal(t, wantEvent.Data(), haveEvent.Data()) // Event data should be unchanged

				// Check that the cloudevent propagation is identical to http propagation
				header := http.Header{}
				otel.GetTextMapPropagator().Inject(spanCtx, propagation.HeaderCarrier(header))
				testSpan.End()
				ext := haveEvent.Extensions()
				require.Equal(t, ext["traceparent"], header.Get("traceparent"))
			},
			Transformers: binding.Transformers{PopulateCEDistributedTracing(spanCtx)},
		},
		{
			Name:       "Check empty span",
			InputEvent: wantEvent,
			AssertFunc: func(t *testing.T, haveEvent event.Event) {
				test.AssertEventEquals(t, wantEvent, haveEvent) // Event should be unchanged
			},
			Transformers: binding.Transformers{PopulateCEDistributedTracing(origCtx)},
		},
	})
}
