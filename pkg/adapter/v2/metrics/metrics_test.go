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
package metrics

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	rectesting "knative.dev/eventing/pkg/reconciler/testing"
)

func TestStatsReporterAdapter(t *testing.T) {
	stats := &rectesting.MockStatsReporter{}
	reporter := NewStatsReporterAdapter(stats)

	event := cloudevents.NewEvent()
	event.SetID("abc-123")
	event.SetSource("unit/test")
	event.SetType("unit.type")

	ctx := ContextWithMetricTag(context.Background(), &MetricTag{
		Name:          "test-name",
		Namespace:     "test-ns",
		ResourceGroup: "test-rg",
	})

	if result := reporter.ReportCount(ctx, event, cloudevents.NewHTTPResult(200, "%w", cloudevents.ResultACK)); !cloudevents.IsACK(result) {
		t.Errorf("unexpected result %v", result)
	}

	if err := stats.ValidateEventCount(1); err != nil {
		t.Errorf("unexpected error %v", err)
	}
}
