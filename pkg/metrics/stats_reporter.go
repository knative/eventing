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
	"log"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

const (
	// anyValue is the default value if the trigger filter attributes are empty.
	AnyValue = "any"
)

var (
	// eventCountM is a counter which records the number of events received
	// by a Trigger.
	eventCountM = stats.Int64(
		"event_count",
		"Number of events received by a Trigger",
		stats.UnitDimensionless,
	)

	// dispatchTimeInMsecM records the time spent dispatching an event to
	// a Trigger subscriber, in milliseconds.
	dispatchTimeInMsecM = stats.Float64(
		"event_dispatch_latencies",
		"The time spent dispatching an event to a Trigger subscriber",
		stats.UnitMilliseconds,
	)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	responseCodeKey      = tag.MustNewKey(LabelResponseCode)
	responseCodeClassKey = tag.MustNewKey(LabelResponseCodeClass)
)

type MetricArgs interface {
	GenerateTag(tags ...tag.Mutator) (context.Context, error)
}

func init() {
	Register([]stats.Measure{}, nil)
}

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCount(args MetricArgs, responseCode int) error
	ReportEventDispatchTime(args MetricArgs, responseCode int, d time.Duration) error
}

var _ StatsReporter = (*reporter)(nil)
var EmptyContext = context.Background()

// reporter holds cached metric objects to report filter metrics.
type reporter struct {
	container  string
	uniqueName string
}

// NewStatsReporter creates a reporter that collects and reports filter metrics.
func NewStatsReporter(container, uniqueName string) StatsReporter {
	return &reporter{
		container:  container,
		uniqueName: uniqueName,
	}
}

func Register(customMetrics []stats.Measure, customViews []*view.View, customTagKeys ...tag.Key) {
	commonTagKeys := []tag.Key{responseCodeKey, responseCodeClassKey, tag.MustNewKey("unique_name"), tag.MustNewKey("container_name")}
	allTagKeys := append(commonTagKeys, customTagKeys...)

	defaultViews := []*view.View{
		{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     allTagKeys,
		},
		{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...),
			TagKeys:     allTagKeys,
		},
	}

	// Add custom views for custom metrics
	for _, metric := range customMetrics {
		defaultViews = append(defaultViews, &view.View{
			Description: metric.Description(),
			Measure:     metric,
			Aggregation: view.LastValue(), // You can change this aggregation as needed
			TagKeys:     allTagKeys,
		})
	}

	// Append custom views
	allViews := append(defaultViews, customViews...)

	if err := metrics.RegisterResourceView(allViews...); err != nil {
		log.Printf("Failed to register opencensus views: %v", err)
	}
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCount(args MetricArgs, responseCode int) error {
	// Create base tags
	baseTags := []tag.Mutator{
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	}

	// Generate context with all tags, including any custom ones from args.GenerateTag
	ctx, err := args.GenerateTag(baseTags...)
	if err != nil {
		return err
	}

	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

// ReportEventDispatchTime captures dispatch times.
func (r *reporter) ReportEventDispatchTime(args MetricArgs, responseCode int, d time.Duration) error {
	// Create base tags
	baseTags := []tag.Mutator{
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)),
	}

	// Generate context with all tags, including any custom ones from args.GenerateTag
	ctx, err := args.GenerateTag(baseTags...)
	if err != nil {
		return err
	}

	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func ValueOrAny(v string) string {
	if v != "" {
		return v
	}
	return AnyValue
}
