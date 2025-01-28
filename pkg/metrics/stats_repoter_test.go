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
package metrics_test

import (
	"context"
	"testing"

	"go.opencensus.io/resource"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	eventingmetrics "knative.dev/eventing/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics/metricstest"
	_ "knative.dev/pkg/metrics/testing"
)

type testMetricArgs struct {
	ns        string
	trigger   string
	broker    string
	testParam string
}

func (args *testMetricArgs) GenerateTag(tags ...tag.Mutator) (context.Context, error) {
	ctx := metricskey.WithResource(context.Background(), resource.Resource{
		Type: eventingmetrics.ResourceTypeKnativeTrigger,
		Labels: map[string]string{
			eventingmetrics.LabelNamespaceName: args.ns,
			eventingmetrics.LabelBrokerName:    args.broker,
			eventingmetrics.LabelTriggerName:   args.trigger,
		},
	})
	ctx, err := tag.New(
		ctx,
		append(tags,
			tag.Insert(customTagKey, args.testParam),
		)...)
	return ctx, err
}

var (
	customMetricM = stats.Int64(
		"custom_metric",
		"A custom metric for testing",
		stats.UnitDimensionless,
	)

	customTagKey = tag.MustNewKey("custom_tag")
)

// StatsReporter interface definition
type StatsReporter interface {
	eventingmetrics.StatsReporter
	ReportCustomMetric(args eventingmetrics.MetricArgs, value int64) error
}

// reporter struct definition
type reporter struct {
	container  string
	uniqueName string
}

// ReportCustomMetric records a custom metric value.
func (r *reporter) ReportCustomMetric(args eventingmetrics.MetricArgs, value int64) error {
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

func TestRegisterCustomMetrics(t *testing.T) {
	setup()

	args := &testMetricArgs{
		ns:        "test-namespace",
		trigger:   "test-trigger",
		broker:    "test-broker",
		testParam: "test-param",
	}
	r := &reporter{
		container:  "testcontainer",
		uniqueName: "testpod",
	}

	customMetrics := []stats.Measure{customMetricM}

	customTagKeys := []tag.Key{customTagKey}

	// No need for customViews, as the custom metric will be used to create the view
	eventingmetrics.Register(customMetrics, nil, customTagKeys...)

	// Verify that the custom view is registered
	if v := view.Find("custom_metric"); v == nil {
		t.Error("Custom view was not registered")
	}

	// Record a value for the custom metric
	expectSuccess(t, func() error {
		return r.ReportCustomMetric(args, 100)
	})

	// Check if the value was recorded correctly
	metricstest.CheckLastValueData(t, "custom_metric", map[string]string{"custom_tag": "test-param"}, 100)
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
	metricstest.Unregister("custom_metric")
	eventingmetrics.Register([]stats.Measure{}, nil)
}
