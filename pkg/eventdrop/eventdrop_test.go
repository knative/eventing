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
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/test/lib/metrics"
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
	r := NewRecorder(mp, nil)

	// Recording the same drop 8 times must aggregate into a single series.
	for range 8 {
		r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	}
	// A drop attributed to a Subscription gets its own series.
	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{
		Subscription: &types.NamespacedName{Namespace: "test-ns", Name: "test-subscription"},
	}, nil)

	metrics.AssertCounter(t, reader, ScopeName, droppedEventsCountName,
		metrics.Point(8, observability.DropReason.With(string(ReasonDeadLetterSinkFailed))),
		metrics.Point(1,
			observability.DropReason.With(string(ReasonDeadLetterSinkFailed)),
			observability.SubscriptionName.With("test-subscription"),
			observability.SubscriptionNamespace.With("test-ns"),
		),
	)
}

func TestRecordEventRejectedMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp, nil)

	r.RecordEventRejected(context.Background(), ReasonTTLExhausted, Labels{
		Broker: &types.NamespacedName{Namespace: "test-ns", Name: "test-broker"},
	}, makeEvent())

	metrics.AssertCounter(t, reader, ScopeName, rejectedEventsCountName, metrics.Point(1,
		observability.DropReason.With(string(ReasonTTLExhausted)),
		observability.BrokerName.With("test-broker"),
		observability.BrokerNamespace.With("test-ns"),
	))
}

func TestRecordEventDroppedDoesNotTouchRejectedCounter(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp, nil)

	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, nil)

	metrics.AssertCounter(t, reader, ScopeName, droppedEventsCountName,
		metrics.Point(1, observability.DropReason.With(string(ReasonDeadLetterSinkFailed))))
	metrics.AssertCounter(t, reader, ScopeName, rejectedEventsCountName)
}

func TestRecordEventSpanRecords(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	r := NewRecorder(nil, tp)

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	r.RecordEventDropped(ctx, ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	r.RecordEventRejected(ctx, ReasonTTLExhausted, Labels{}, makeEvent())
	span.End()

	records := map[string][]attribute.KeyValue{}
	for _, s := range exporter.GetSpans() {
		if s.Name != droppedSpanName && s.Name != rejectedSpanName {
			continue
		}
		records[s.Name] = s.Attributes
	}
	if len(records) != 2 {
		t.Fatalf("expected a dropped and a rejected span record, got %v", records)
	}

	for name, attrs := range records {
		got := map[string]string{}
		for _, kv := range attrs {
			got[string(kv.Key)] = kv.Value.Emit()
		}

		wantReason := string(ReasonTTLExhausted)
		if name == droppedSpanName {
			wantReason = string(ReasonDeadLetterSinkFailed)
		}
		for key, want := range map[string]string{
			string(observability.DropReason):     wantReason,
			string(observability.CloudEventID):   "test-id",
			string(observability.CloudEventType): "test-type",
		} {
			if got[key] != want {
				t.Errorf("span record %q: expected attribute %s=%q, got %q", name, key, want, got[key])
			}
		}
	}
}

func TestRecordNoop(t *testing.T) {
	// A nil Recorder and a Recorder without a meter provider must neither
	// create counters nor panic.
	var nilRecorder *Recorder
	nilRecorder.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	nilRecorder.RecordEventRejected(context.Background(), ReasonTTLMissing, Labels{}, makeEvent())

	r := NewRecorder(nil, nil)
	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	r.RecordEventRejected(context.Background(), ReasonTTLMissing, Labels{}, makeEvent())

	if r.droppedEvents != nil || r.rejectedEvents != nil {
		t.Error("expected no counters to be created without a meter provider")
	}
}

func TestRecordEventDroppedWithoutActiveSpan(t *testing.T) {
	// Deliveries continuing in the background after the request span ended
	// must still produce a trace record: a short-lived span of its own.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	r := NewRecorder(nil, tp)

	// The request context outlives its span, like a background delivery after
	// the request already ended.
	ctx, span := tp.Tracer("test").Start(context.Background(), "ended-request")
	span.End()

	r.RecordEventDropped(ctx, ReasonDeadLetterSinkFailed,
		Labels{Subscription: &types.NamespacedName{Namespace: "test-ns", Name: "test-subscription"}}, makeEvent())

	spans := exporter.GetSpans()
	var drop *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == droppedSpanName {
			drop = &spans[i]
		}
	}
	if drop == nil {
		t.Fatalf("expected a %q span, got %+v", droppedSpanName, spans)
	}

	attrs := map[string]string{}
	for _, kv := range drop.Attributes {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	for key, want := range map[string]string{
		string(observability.DropReason):            string(ReasonDeadLetterSinkFailed),
		string(observability.SubscriptionName):      "test-subscription",
		string(observability.SubscriptionNamespace): "test-ns",
		string(observability.CloudEventID):          "test-id",
	} {
		if got := attrs[key]; got != want {
			t.Errorf("drop span: expected attribute %s=%q, got %q", key, want, got)
		}
	}
}

func TestSubscriptionDropSeriesByNamespace(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp, nil)

	for _, ns := range []string{"ns-a", "ns-b", ""} {
		r.RecordEventDropped(t.Context(), ReasonDeadLetterSinkFailed, Labels{
			Subscription: &types.NamespacedName{Name: "test-subscription", Namespace: ns},
		}, nil)
	}

	reason := observability.DropReason.With(string(ReasonDeadLetterSinkFailed))
	name := observability.SubscriptionName.With("test-subscription")
	metrics.AssertCounter(t, reader, ScopeName, droppedEventsCountName,
		metrics.Point(1, reason, name, observability.SubscriptionNamespace.With("ns-a")),
		metrics.Point(1, reason, name, observability.SubscriptionNamespace.With("ns-b")),
		metrics.Point(1, reason, name),
	)
}

func TestNewRecorderOmitsFailedCounterOnly(t *testing.T) {
	// A counter whose instrument creation fails must be omitted without
	// disabling its sibling: the OTel SDK may return a non-nil handle together
	// with an error, so the error must be honored.
	for _, bad := range []string{droppedEventsCountName, rejectedEventsCountName} {
		t.Run(bad, func(t *testing.T) {
			reader := metric.NewManualReader()
			// One view renaming the instrument and one with an aggregation
			// incompatible with a counter make the SDK fail instrument
			// creation while still returning a handle.
			mp := metric.NewMeterProvider(metric.WithReader(reader), metric.WithView(
				metric.NewView(metric.Instrument{Name: bad}, metric.Stream{Name: "partial.sum", Aggregation: metric.AggregationSum{}}),
				metric.NewView(metric.Instrument{Name: bad}, metric.Stream{Name: "invalid.aggregation", Aggregation: metric.AggregationLastValue{}}),
			))

			if handle, err := mp.Meter(ScopeName).Int64Counter(bad); err == nil || handle == nil {
				t.Fatalf("control: expected the SDK to return both an error and a handle, got handle=%v err=%v", handle, err)
			}

			rec := NewRecorder(mp, nil)
			failedRetained := rec.droppedEvents != nil
			if bad == rejectedEventsCountName {
				failedRetained = rec.rejectedEvents != nil
			}
			if failedRetained {
				t.Error("expected the failed counter to be omitted")
			}

			rec.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, nil)
			rec.RecordEventRejected(context.Background(), ReasonTTLMissing, Labels{}, nil)

			sums := map[string]int64{}
			var rm metricdata.ResourceMetrics
			if err := reader.Collect(t.Context(), &rm); err != nil {
				t.Fatal(err)
			}
			for _, sm := range rm.ScopeMetrics {
				for _, m := range sm.Metrics {
					for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
						sums[m.Name] += dp.Value
					}
				}
			}
			if sums["partial.sum"] != 0 {
				t.Errorf("expected nothing recorded on the failed counter, got %d", sums["partial.sum"])
			}
			healthy := rejectedEventsCountName
			if bad == rejectedEventsCountName {
				healthy = droppedEventsCountName
			}
			if sums[healthy] != 1 {
				t.Errorf("expected the healthy counter to record 1, got %d", sums[healthy])
			}
		})
	}
}
