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
	"fmt"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"knative.dev/pkg/source"
)

type StatsReporterAdapter interface {
	ReportCount(ctx context.Context, event cloudevents.Event, err error) error
}

type statsReporterAdapter struct {
	source.StatsReporter
}

func NewStatsReporterAdapter(reporter source.StatsReporter) StatsReporterAdapter {
	return &statsReporterAdapter{StatsReporter: reporter}
}

func (c *statsReporterAdapter) ReportCount(ctx context.Context, event cloudevents.Event, err error) error {
	tags := MetricTagFromContext(ctx)
	reportArgs := &source.ReportArgs{
		Namespace:     tags.Namespace,
		EventSource:   event.Source(),
		EventType:     event.Type(),
		Name:          tags.Name,
		ResourceGroup: tags.ResourceGroup,
	}
	if rErr := c.ReportEventCount(reportArgs, StatusCode(err)); rErr != nil {
		// metrics is not important enough to return an error if it is setup wrong.
		// So combine reporter error with ce error if not nil.
		if err != nil {
			err = fmt.Errorf("%w\nmetrics reporter errror: %s", err, rErr)
		}
	}
	return err
}

func StatusCode(err error) int {
	// TODO: there is talk to change this in the sdk, for now we do not have access to the real http code.
	if err != nil {
		return 400
	}
	return 200
}

// Metric context

type MetricTag struct {
	Name          string
	Namespace     string
	ResourceGroup string
}

type metricKey struct{}

// ContextWithMetricTag returns a copy of parent context in which the
// value associated with metric key is the supplied metric tag.
func ContextWithMetricTag(ctx context.Context, metric *MetricTag) context.Context {
	return context.WithValue(ctx, metricKey{}, metric)
}

// MetricTagFromContext returns the metric tag stored in context.
// Returns nil if no metric tag is set in context, or if the stored value is
// not of correct type.
func MetricTagFromContext(ctx context.Context) *MetricTag {
	if logger, ok := ctx.Value(metricKey{}).(*MetricTag); ok {
		return logger
	}
	return &MetricTag{
		Name:          "unknown",
		Namespace:     "unknown",
		ResourceGroup: "unknown",
	}
}
