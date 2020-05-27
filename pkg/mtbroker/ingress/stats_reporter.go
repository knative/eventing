/*
 * Copyright 2020 The Knative Authors
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
	"log"
	"strconv"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	broker "knative.dev/eventing/pkg/mtbroker"
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

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	namespaceKey         = tag.MustNewKey(metricskey.LabelNamespaceName)
	brokerKey            = tag.MustNewKey(metricskey.LabelBrokerName)
	eventTypeKey         = tag.MustNewKey(metricskey.LabelEventType)
	responseCodeKey      = tag.MustNewKey(metricskey.LabelResponseCode)
	responseCodeClassKey = tag.MustNewKey(metricskey.LabelResponseCodeClass)
)

type ReportArgs struct {
	ns        string
	broker    string
	eventType string
}

func init() {
	register()
}

// StatsReporter defines the interface for sending ingress metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, responseCode int) error
	ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error
}

var _ StatsReporter = (*reporter)(nil)
var emptyContext = context.Background()

// Reporter holds cached metric objects to report ingress metrics.
type reporter struct {
	container  string
	uniqueName string
}

// NewStatsReporter creates a reporter that collects and reports ingress metrics.
func NewStatsReporter(container, uniqueName string) StatsReporter {
	return &reporter{
		container:  container,
		uniqueName: uniqueName,
	}
}

func register() {
	tagKeys := []tag.Key{
		namespaceKey,
		brokerKey,
		eventTypeKey,
		responseCodeKey,
		responseCodeClassKey,
		broker.ContainerTagKey,
		broker.UniqueTagKey}

	// Create view to see our measurements.
	err := view.Register(
		&view.View{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Description: dispatchTimeInMsecM.Description(),
			Measure:     dispatchTimeInMsecM,
			Aggregation: view.Distribution(metrics.Buckets125(1, 10000)...), // 1, 2, 5, 10, 20, 50, 100, 500, 1000, 5000, 10000
			TagKeys:     tagKeys,
		},
	)
	if err != nil {
		log.Printf("failed to register opencensus views, %s", err)
	}
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
		emptyContext,
		tag.Insert(broker.ContainerTagKey, r.container),
		tag.Insert(broker.UniqueTagKey, r.uniqueName),
		tag.Insert(namespaceKey, args.ns),
		tag.Insert(brokerKey, args.broker),
		tag.Insert(eventTypeKey, args.eventType),
		tag.Insert(responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(responseCodeClassKey, metrics.ResponseCodeClass(responseCode)))
}
