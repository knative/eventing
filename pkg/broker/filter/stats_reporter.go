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

package filter

import (
	"context"
	"log"
	"strconv"
	"time"

	"go.opencensus.io/resource"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	broker "knative.dev/eventing/pkg/broker"
	eventingmetrics "knative.dev/eventing/pkg/metrics"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
)

const (
	// anyValue is the default value if the trigger filter attributes are empty.
	anyValue = "any"
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

	// processingTimeInMsecM records the time spent between arrival at the Broker
	// and the delivery to the Trigger subscriber.
	processingTimeInMsecM = stats.Float64(
		"event_processing_latencies",
		"The time spent processing an event before it is dispatched to a Trigger subscriber",
		stats.UnitMilliseconds,
	)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	triggerFilterTypeKey        = tag.MustNewKey(eventingmetrics.LabelFilterType)
	triggerFilterRequestTypeKey = tag.MustNewKey("filter_request_type")
	responseCodeKey             = tag.MustNewKey(eventingmetrics.LabelResponseCode)
	responseCodeClassKey        = tag.MustNewKey(eventingmetrics.LabelResponseCodeClass)
)

type ReportArgs struct {
	ns          string
	trigger     string
	broker      string
	filterType  string
	requestType string
}

func init() {
	register()
}

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, responseCode int) error
	ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error
	ReportEventProcessingTime(args *ReportArgs, d time.Duration) error
}

var _ StatsReporter = (*reporter)(nil)
var emptyContext = context.Background()

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

func register() {
	// Create view to see our measurements.
	err := metrics.RegisterResourceView(
		&view.View{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{triggerFilterTypeKey, triggerFilterRequestTypeKey, responseCodeKey, responseCodeClassKey, broker.UniqueTagKey, broker.ContainerTagKey},
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys:     []tag.Key{triggerFilterTypeKey, triggerFilterRequestTypeKey, responseCodeKey, responseCodeClassKey, broker.UniqueTagKey, broker.ContainerTagKey},
		},
		&view.View{
			Description: processingTimeInMsecM.Description(),
			Measure:     processingTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 1000, 5000, 10000
			TagKeys:     []tag.Key{triggerFilterTypeKey, triggerFilterRequestTypeKey, broker.UniqueTagKey, broker.ContainerTagKey},
		},
	)
	if err != nil {
		log.Printf("failed to register opencensus views, %s", err)
	}
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	ctx, err := r.generateTag(args,
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)))
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

// ReportEventDispatchTime captures dispatch times.
func (r *reporter) ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error {
	ctx, err := r.generateTag(args,
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)))
	if err != nil {
		return err
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

// ReportEventProcessingTime captures event processing times.
func (r *reporter) ReportEventProcessingTime(args *ReportArgs, d time.Duration) error {
	ctx, err := r.generateTag(args)
	if err != nil {
		return err
	}

	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, processingTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, tags ...tag.Mutator) (context.Context, error) {
	ctx := metricskey.WithResource(emptyContext, resource.Resource{
		Type: eventingmetrics.ResourceTypeKnativeTrigger,
		Labels: map[string]string{
			eventingmetrics.LabelNamespaceName: args.ns,
			eventingmetrics.LabelBrokerName:    args.broker,
			eventingmetrics.LabelTriggerName:   args.trigger,
		},
	})
	// Note that filterType and filterSource can be empty strings, so they need a special treatment.
	ctx, err := tag.New(
		ctx,
		append(tags,
			tag.Insert(broker.ContainerTagKey, r.container),
			tag.Insert(broker.UniqueTagKey, r.uniqueName),
			tag.Insert(triggerFilterTypeKey, valueOrAny(args.filterType)),
			tag.Insert(triggerFilterRequestTypeKey, args.requestType),
		)...)
	return ctx, err
}

func valueOrAny(v string) string {
	if v != "" {
		return v
	}
	return anyValue
}
