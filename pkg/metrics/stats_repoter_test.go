package metrics

import (
	"context"
	"net/http"
	"testing"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics/metricstest"
	_ "knative.dev/pkg/metrics/testing"
)

type testMetricArgs struct{}

func (t *testMetricArgs) GenerateTag(tags ...tag.Mutator) (context.Context, error) {
	return tag.New(EmptyContext, append(tags,
		tag.Insert(tag.MustNewKey("unique_name"), "testpod"),
		tag.Insert(tag.MustNewKey("container_name"), "testcontainer"),
	)...)
}

// Custom metric for testing
var customMetricM = stats.Int64(
	"custom_metric",
	"A custom metric for testing",
	stats.UnitDimensionless,
)

// Custom tag for testing
var customTagKey = tag.MustNewKey("custom_tag")

// add ReportEventCountRetry to StatsReporter
type statsReporterTester interface {
	StatsReporter
	ReportCustomMetric(args MetricArgs, value int64) error
}

// ReportCustomMetric records a custom metric value.
func (r *reporter) ReportCustomMetric(args MetricArgs, value int64) error {
	// Create base tags
	baseTags := []tag.Mutator{
		tag.Insert(tag.MustNewKey("unique_name"), r.uniqueName),
		tag.Insert(tag.MustNewKey("container_name"), r.container),
	}

	// Generate context with all tags, including any custom ones from args.GenerateTag
	ctx, err := args.GenerateTag(baseTags...)
	if err != nil {
		return err
	}

	// Record the custom metric
	stats.Record(ctx, customMetricM.M(value))
	return nil
}
func NewStatsReporterTester(container, uniqueName string) statsReporterTester {
	return &reporter{
		container:  container,
		uniqueName: uniqueName,
	}
}

func TestStatsReporter(t *testing.T) {
	setup()

	args := &testMetricArgs{}
	r := NewStatsReporterTester("testcontainer", "testpod")

	wantTags := map[string]string{
		"response_code":       "202",
		"response_code_class": "2xx",
		"unique_name":         "testpod",
		"container_name":      "testcontainer",
	}

	// Test ReportEventCount
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	expectSuccess(t, func() error {
		return r.ReportEventCount(args, http.StatusAccepted)
	})
	metricstest.CheckCountData(t, "event_count", wantTags, 2)

	// Test ReportEventDispatchTime
	expectSuccess(t, func() error {
		return r.ReportEventDispatchTime(args, http.StatusAccepted, 1100*time.Millisecond)
	})
	expectSuccess(t, func() error {
		return r.ReportEventDispatchTime(args, http.StatusAccepted, 9100*time.Millisecond)
	})
	metricstest.CheckDistributionData(t, "event_dispatch_latencies", wantTags, 2, 1100.0, 9100.0)

}

func TestRegisterCustomMetrics(t *testing.T) {
	setup()

	customMetrics := []stats.Measure{customMetricM}
	customViews := []*view.View{
		{
			Description: customMetricM.Description(),
			Measure:     customMetricM,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{customTagKey},
		},
	}
	customTagKeys := []tag.Key{customTagKey}

	Register(customMetrics, customViews, customTagKeys...)

	// Verify that the custom view is registered
	if v := view.Find("custom_metric"); v == nil {
		t.Error("Custom view was not registered")
	}

	// Record a value for the custom metric
	ctx, _ := tag.New(context.Background(), tag.Insert(customTagKey, "test_value"))
	stats.Record(ctx, customMetricM.M(100))

	// Check if the value was recorded correctly
	metricstest.CheckLastValueData(t, "custom_metric", map[string]string{"custom_tag": "test_value"}, 100)
}

func expectSuccess(t *testing.T, f func() error) {
	t.Helper()
	if err := f(); err != nil {
		t.Errorf("Reporter expected success but got error: %v", err)
	}
}

func setup() {
	resetMetrics()
}

func resetMetrics() {
	metricstest.Unregister(
		"event_count",
		"event_dispatch_latencies",
		"event_processing_latencies",
		"custom_metric")
	Register([]stats.Measure{}, nil)
}
