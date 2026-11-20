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

// Package metrics provides assertions for collected OpenTelemetry metrics.
package metrics

import (
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

// Point describes one counter series with its cumulative value and exact attributes.
func Point(value int64, attrs ...attribute.KeyValue) metricdata.DataPoint[int64] {
	return metricdata.DataPoint[int64]{Attributes: attribute.NewSet(attrs...), Value: value}
}

// AssertCounter checks every series of the named counter. With no expected
// points it checks that the counter was absent from the collection.
func AssertCounter(t *testing.T, reader *metric.ManualReader, scope, name string, want ...metricdata.DataPoint[int64]) {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(t.Context(), &rm); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name != scope {
			continue
		}
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if len(want) == 0 || found {
				t.Fatalf("unexpected counter %q in scope %q", name, scope)
			}
			found = true
			metricdatatest.AssertAggregationsEqual(t, metricdata.Sum[int64]{
				DataPoints:  want,
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
			}, m.Data, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreExemplars())
		}
	}
	if len(want) > 0 && !found {
		t.Errorf("counter %q missing from scope %q", name, scope)
	}
}
