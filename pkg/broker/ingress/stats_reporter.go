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

package ingress

import (
	"context"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	. "knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
)

var (
	// eventCountM is a counter which records the number of events received
	// by the Broker.
	eventCountM = stats.Int64(
		"event_count",
		"Number of events received by a Broker",
		stats.UnitDimensionless,
	)

	// dispatchTimeInMsecM records the time spent dispatching an event to
	// a Channel, in milliseconds.
	dispatchTimeInMsecM = stats.Float64(
		"event_dispatch_latencies",
		"The time spent dispatching an event to a Channel",
		stats.UnitMilliseconds,
	)
)

type ReportArgs struct {
	ns          string
	broker      string
	eventType   string
	eventSource string
}

// StatsReporter defines the interface for sending ingress metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, responseCode int) error
	ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error
}

var _ StatsReporter = (*reporter)(nil)

// Reporter holds cached metric objects to report ingress metrics.
type reporter struct {
	namespaceTagKey      tag.Key
	brokerTagKey         tag.Key
	eventTypeKey         tag.Key
	eventSourceKey       tag.Key
	responseCodeKey      tag.Key
	responseCodeClassKey tag.Key
}

// NewStatsReporter creates a reporter that collects and reports ingress metrics.
func NewStatsReporter() (StatsReporter, error) {
	var r = &reporter{}

	// Create the tag keys that will be used to add tags to our measurements.
	nsTag, err := tag.NewKey(metricskey.LabelNamespaceName)
	if err != nil {
		return nil, err
	}
	r.namespaceTagKey = nsTag
	brokerTag, err := tag.NewKey(metricskey.LabelBrokerName)
	if err != nil {
		return nil, err
	}
	r.brokerTagKey = brokerTag
	eventTypeTag, err := tag.NewKey(metricskey.LabelEventType)
	if err != nil {
		return nil, err
	}
	r.eventTypeKey = eventTypeTag
	eventSourceTag, err := tag.NewKey(metricskey.LabelEventSource)
	if err != nil {
		return nil, err
	}
	r.eventSourceKey = eventSourceTag
	responseCodeTag, err := tag.NewKey(LabelResponseCode)
	if err != nil {
		return nil, err
	}
	r.responseCodeKey = responseCodeTag
	responseCodeClassTag, err := tag.NewKey(LabelResponseCodeClass)
	if err != nil {
		return nil, err
	}
	r.responseCodeClassKey = responseCodeClassTag

	// Create view to see our measurements.
	err = view.Register(
		&view.View{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.namespaceTagKey, r.brokerTagKey, r.eventTypeKey, r.eventSourceKey, r.responseCodeKey, r.responseCodeClassKey},
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100
			TagKeys:     []tag.Key{r.namespaceTagKey, r.brokerTagKey, r.eventTypeKey, r.eventSourceKey, r.responseCodeKey, r.responseCodeClassKey},
		},
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	ctx, err := r.generateTag(args, responseCode)
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

// ReportEventDispatchTime captures dispatch times.
func (r *reporter) ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error {
	ctx, err := r.generateTag(args, responseCode)
	if err != nil {
		return err
	}
	// convert time.Duration in nanoseconds to milliseconds.
	metrics.Record(ctx, dispatchTimeInMsecM.M(float64(d/time.Millisecond)))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, responseCode int) (context.Context, error) {
	return tag.New(
		context.Background(),
		tag.Insert(r.namespaceTagKey, args.ns),
		tag.Insert(r.brokerTagKey, args.broker),
		tag.Insert(r.eventTypeKey, args.eventType),
		tag.Insert(r.eventSourceKey, args.eventSource),
		tag.Insert(r.responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(r.responseCodeClassKey, utils.ResponseCodeClass(responseCode)))
}
