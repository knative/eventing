/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package filter

import (
	"context"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	utils "knative.dev/eventing/pkg/broker"
	. "knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
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
)

type ReportArgs struct {
	ns           string
	trigger      string
	broker       string
	filterType   string
	filterSource string
}

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, err error) error
	ReportEventDispatchTime(args *ReportArgs, err error, d time.Duration) error
	ReportEventProcessingTime(args *ReportArgs, err error, d time.Duration) error
}

var _ StatsReporter = (*reporter)(nil)

// reporter holds cached metric objects to report filter metrics.
type reporter struct {
	namespaceTagKey        tag.Key
	triggerTagKey          tag.Key
	brokerTagKey           tag.Key
	triggerFilterTypeKey   tag.Key
	triggerFilterSourceKey tag.Key
	resultKey              tag.Key
	filterResultKey        tag.Key
}

// NewStatsReporter creates a reporter that collects and reports filter metrics.
func NewStatsReporter() (StatsReporter, error) {
	var r = &reporter{}

	// Create the tag keys that will be used to add tags to our measurements.
	nsTag, err := tag.NewKey(metricskey.LabelNamespaceName)
	if err != nil {
		return nil, err
	}
	r.namespaceTagKey = nsTag
	triggerTag, err := tag.NewKey(metricskey.LabelTriggerName)
	if err != nil {
		return nil, err
	}
	r.triggerTagKey = triggerTag
	brokerTag, err := tag.NewKey(metricskey.LabelBrokerName)
	if err != nil {
		return nil, err
	}
	r.brokerTagKey = brokerTag
	triggerFilterTypeTag, err := tag.NewKey(metricskey.LabelFilterType)
	if err != nil {
		return nil, err
	}
	r.triggerFilterTypeKey = triggerFilterTypeTag
	triggerFilterSourceKey, err := tag.NewKey(metricskey.LabelFilterSource)
	if err != nil {
		return nil, err
	}
	r.triggerFilterSourceKey = triggerFilterSourceKey
	filterResultTag, err := tag.NewKey(LabelFilterResult)
	if err != nil {
		return nil, err
	}
	r.filterResultKey = filterResultTag
	resultTag, err := tag.NewKey(LabelResult)
	if err != nil {
		return nil, err
	}
	r.resultKey = resultTag

	// Create view to see our measurements.
	err = view.Register(
		&view.View{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.namespaceTagKey, r.triggerTagKey, r.brokerTagKey, r.triggerFilterTypeKey, r.triggerFilterSourceKey, r.resultKey},
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(utils.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100
			TagKeys:     []tag.Key{r.namespaceTagKey, r.triggerTagKey, r.brokerTagKey, r.triggerFilterTypeKey, r.triggerFilterSourceKey, r.resultKey},
		},
		&view.View{
			Description: processingTimeInMsecM.Description(),
			Measure:     processingTimeInMsecM,
			Aggregation: view.Distribution(utils.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100
			TagKeys:     []tag.Key{r.namespaceTagKey, r.triggerTagKey, r.brokerTagKey, r.triggerFilterTypeKey, r.triggerFilterSourceKey, r.resultKey},
		},
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCount(args *ReportArgs, err error) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, utils.Result(err)))
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

// ReportEventDispatchTime captures dispatch times.
func (r *reporter) ReportEventDispatchTime(args *ReportArgs, err error, d time.Duration) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, utils.Result(err)))
	if err != nil {
		return err
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

// ReportEventProcessingTime captures event processing times.
func (r *reporter) ReportEventProcessingTime(args *ReportArgs, err error, d time.Duration) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, utils.Result(err)))
	if err != nil {
		return err
	}

	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, processingTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, t tag.Mutator) (context.Context, error) {
	// Note that filterType and filterSource can be empty strings, so they need a special treatment.
	return tag.New(
		context.Background(),
		tag.Insert(r.namespaceTagKey, args.ns),
		tag.Insert(r.triggerTagKey, args.trigger),
		tag.Insert(r.brokerTagKey, args.broker),
		tag.Insert(r.triggerFilterTypeKey, valueOrAny(args.filterType)),
		tag.Insert(r.triggerFilterSourceKey, valueOrAny(args.filterSource)),
		t)
}

func valueOrAny(v string) string {
	if v != "" {
		return v
	}
	return AnyValue
}
