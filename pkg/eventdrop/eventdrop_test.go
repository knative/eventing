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
	"slices"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/eventing/pkg/observability"
)

func makeEvent() *cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetID("test-id")
	event.SetType("test-type")
	event.SetSource("test-source")
	return &event
}

// seriesExpectation describes one expected time series of a counter: the
// exact attribute set and the cumulative sum.
type seriesExpectation struct {
	attrs []attribute.KeyValue
	sum   int64
}

// assertCounter asserts the counter with the given name exists exactly once
// and consists of exactly the expected time series, each with the exact
// expected attribute set and cumulative sum. This catches counting errors as
// well as unexpected (e.g. high-cardinality, per-event) attributes.
func assertCounter(t *testing.T, rm *metricdata.ResourceMetrics, name string, want ...seriesExpectation) {
	t.Helper()

	found := false
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name != ScopeName {
			continue
		}
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			found = true

			data, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				t.Fatalf("metric %s: expected Sum[int64], got %T", name, m.Data)
			}
			if len(data.DataPoints) != len(want) {
				t.Fatalf("metric %s: expected %d time series, got %d: %v", name, len(want), len(data.DataPoints), data.DataPoints)
			}

			got := map[string]int64{}
			for _, dp := range data.DataPoints {
				got[encodeAttrs(dp.Attributes.ToSlice())] += dp.Value
			}
			wantSeries := map[string]int64{}
			for _, w := range want {
				wantSeries[encodeAttrs(w.attrs)] = w.sum
			}
			for key, wantSum := range wantSeries {
				if gotSum, ok := got[key]; !ok {
					t.Errorf("metric %s: missing series %s", name, key)
				} else if gotSum != wantSum {
					t.Errorf("metric %s: series %s: expected sum %d, got %d", name, key, wantSum, gotSum)
				}
			}
			for key := range got {
				if _, ok := wantSeries[key]; !ok {
					t.Errorf("metric %s: unexpected series %s", name, key)
				}
			}
		}
	}
	if !found {
		t.Fatalf("metric %s was not recorded", name)
	}
}

func encodeAttrs(attrs []attribute.KeyValue) string {
	sorted := make([]attribute.KeyValue, len(attrs))
	copy(sorted, attrs)
	slices.SortFunc(sorted, func(a, b attribute.KeyValue) int {
		return strings.Compare(string(a.Key), string(b.Key))
	})
	parts := make([]string, len(sorted))
	for i, kv := range sorted {
		parts[i] = string(kv.Key) + "=" + kv.Value.Emit()
	}
	return strings.Join(parts, ",")
}

func collect(t *testing.T, reader *metric.ManualReader) *metricdata.ResourceMetrics {
	t.Helper()
	rm := &metricdata.ResourceMetrics{}
	if err := reader.Collect(context.Background(), rm); err != nil {
		t.Fatal("failed to collect metrics:", err)
	}
	return rm
}

func TestRecordEventDroppedMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp)

	// Recording the same drop 8 times must aggregate into a single series.
	for range 8 {
		r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	}
	// A drop attributable to a trigger gets its own series.
	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{
		Trigger: &types.NamespacedName{Namespace: "test-ns", Name: "test-trigger"},
	}, nil)

	assertCounter(t, collect(t, reader), droppedEventsCountName,
		seriesExpectation{
			attrs: []attribute.KeyValue{observability.DropReason.With(string(ReasonDeadLetterSinkFailed))},
			sum:   8,
		},
		seriesExpectation{
			attrs: []attribute.KeyValue{
				observability.DropReason.With(string(ReasonDeadLetterSinkFailed)),
				observability.TriggerName.With("test-trigger"),
				observability.TriggerNamespace.With("test-ns"),
			},
			sum: 1,
		},
	)
}

func TestRecordEventRejectedMetrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp)

	r.RecordEventRejected(context.Background(), ReasonTTLExhausted, Labels{
		Broker: &types.NamespacedName{Namespace: "test-ns", Name: "test-broker"},
	}, makeEvent())

	assertCounter(t, collect(t, reader), rejectedEventsCountName,
		seriesExpectation{
			attrs: []attribute.KeyValue{
				observability.DropReason.With(string(ReasonTTLExhausted)),
				observability.BrokerName.With("test-broker"),
				observability.BrokerNamespace.With("test-ns"),
			},
			sum: 1,
		},
	)
}

func TestRecordEventDroppedDoesNotTouchRejectedCounter(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	r := NewRecorder(mp)

	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, nil)

	rm := collect(t, reader)
	assertCounter(t, rm, droppedEventsCountName,
		seriesExpectation{
			attrs: []attribute.KeyValue{observability.DropReason.With(string(ReasonDeadLetterSinkFailed))},
			sum:   1,
		})
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == rejectedEventsCountName {
				t.Errorf("metric %s was recorded, expected none", rejectedEventsCountName)
			}
		}
	}
}

func TestRecordEventSpanEvents(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	r := NewRecorder(nil)

	ctx, span := tp.Tracer("test").Start(context.Background(), "parent")
	r.RecordEventDropped(ctx, ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	r.RecordEventRejected(ctx, ReasonTTLExhausted, Labels{}, makeEvent())
	span.End()

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if len(spans[0].Events) != 2 {
		t.Fatalf("expected 2 span events, got %d", len(spans[0].Events))
	}

	for _, e := range spans[0].Events {
		attrs := map[string]string{}
		for _, kv := range e.Attributes {
			attrs[string(kv.Key)] = kv.Value.Emit()
		}

		var wantReason string
		switch e.Name {
		case droppedEventSpanEvent:
			wantReason = string(ReasonDeadLetterSinkFailed)
		case rejectedEventSpanEvent:
			wantReason = string(ReasonTTLExhausted)
		default:
			t.Errorf("unexpected span event %q", e.Name)
			continue
		}

		for key, want := range map[string]string{
			string(observability.DropReason):     wantReason,
			string(observability.CloudEventID):   "test-id",
			string(observability.CloudEventType): "test-type",
		} {
			if got := attrs[key]; got != want {
				t.Errorf("span event %q: expected attribute %s=%q, got %q", e.Name, key, want, got)
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

	r := NewRecorder(nil)
	r.RecordEventDropped(context.Background(), ReasonDeadLetterSinkFailed, Labels{}, makeEvent())
	r.RecordEventRejected(context.Background(), ReasonTTLMissing, Labels{}, makeEvent())

	if r.droppedEvents != nil || r.rejectedEvents != nil {
		t.Error("expected no counters to be created without a meter provider")
	}
}
