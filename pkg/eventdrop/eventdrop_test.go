/*
Copyright 2026 The Knative Authors

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

package eventdrop

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/observability"
	"knative.dev/pkg/observability/metrics/metricstest"
)

func makeEvent() *cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID("test-id")
	event.SetType("test-type")
	event.SetSource("test-source")
	return &event
}

func TestRecordEventDroppedMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp)

	r.RecordEventDropped(context.Background(), ReasonTTLExhausted, Labels{
		Broker: &types.NamespacedName{Namespace: "test-ns", Name: "test-broker"},
	}, makeEvent())

	metricstest.AssertMetrics(t, reader,
		metricstest.MetricsPresent(ScopeName, droppedEventsCountName),
		metricstest.HasAttributes(ScopeName, droppedEventsCountName,
			observability.DropReason.With(string(ReasonTTLExhausted)),
			observability.BrokerName.With("test-broker"),
			observability.BrokerNamespace.With("test-ns"),
		),
	)
}

func TestRecordEventDroppedMetricsWithTriggerLabels(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp)

	r.RecordEventDropped(context.Background(), ReasonTTLMissing, Labels{
		Trigger: &types.NamespacedName{Namespace: "test-ns", Name: "test-trigger"},
	}, nil)

	metricstest.AssertMetrics(t, reader,
		metricstest.HasAttributes(ScopeName, droppedEventsCountName,
			observability.DropReason.With(string(ReasonTTLMissing)),
			observability.TriggerName.With("test-trigger"),
			observability.TriggerNamespace.With("test-ns"),
		),
	)
}

func TestRecordEventDroppedSpanEvent(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	r := NewRecorder(nil)

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	r.RecordEventDropped(ctx, ReasonTTLExhausted, Labels{}, makeEvent())
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if len(spans[0].Events) != 1 {
		t.Fatalf("expected 1 span event, got %d", len(spans[0].Events))
	}

	e := spans[0].Events[0]
	if e.Name != droppedEventSpanEventName {
		t.Errorf("expected span event %q, got %q", droppedEventSpanEventName, e.Name)
	}

	attrs := map[string]string{}
	for _, kv := range e.Attributes {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	for key, want := range map[string]string{
		string(observability.DropReason):     string(ReasonTTLExhausted),
		string(observability.CloudEventID):   "test-id",
		string(observability.CloudEventType): "test-type",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("expected span event attribute %s=%q, got %q", key, want, got)
		}
	}
}

func TestRecordEventDroppedNoop(t *testing.T) {
	// A nil Recorder and a Recorder without a meter provider must not record
	// anything nor panic.
	var nilRecorder *Recorder
	nilRecorder.RecordEventDropped(context.Background(), ReasonTTLMissing, Labels{}, makeEvent())

	reader := metric.NewManualReader()
	NewRecorder(nil).RecordEventDropped(context.Background(), ReasonTTLMissing, Labels{}, makeEvent())

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	if len(rm.ScopeMetrics) != 0 {
		t.Errorf("expected no metrics to be recorded, got %+v", rm.ScopeMetrics)
	}
}
