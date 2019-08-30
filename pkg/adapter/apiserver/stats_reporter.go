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

package apiserver

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics"
)

var (
	// eventCountM is a counter which records the number of events sent
	// by an Importer.
	eventCountM = stats.Int64(
		"event_count",
		"Number of events created",
		stats.UnitDimensionless,
	)
)

type ReportArgs struct {
	ns          string
	eventType   string
	eventSource string
}

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, err error) error
}

var _ StatsReporter = (*reporter)(nil)

// reporter holds cached metric objects to report filter metrics.
type reporter struct {
	namespaceTagKey tag.Key
	eventTypeKey    tag.Key
	eventSourceKey  tag.Key
	resultKey       tag.Key
}

// NewStatsReporter creates a reporter that collects and reports apiserversource
// metrics.
func NewStatsReporter() (StatsReporter, error) {
	var r = &reporter{}

	// Create the tag keys that will be used to add tags to our measurements.
	nsTag, err := tag.NewKey(metricskey.NamespaceName)
	if err != nil {
		return nil, err
	}
	r.namespaceTagKey = nsTag
	eventTypeTag, err := tag.NewKey(metricskey.EventType)
	if err != nil {
		return nil, err
	}
	r.eventTypeKey = eventTypeTag
	eventSourceTag, err := tag.NewKey(metricskey.EventSource)
	if err != nil {
		return nil, err
	}
	r.eventSourceKey = eventSourceTag
	resultTag, err := tag.NewKey(metricskey.Result)
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
			TagKeys: []tag.Key{r.namespaceTagKey, r.eventSourceKey,
				r.eventTypeKey},
		},
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCount(args *ReportArgs, err error) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, Result(err)))
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, t tag.Mutator) (context.Context, error) {
	return tag.New(
		context.Background(),
		tag.Insert(r.namespaceTagKey, args.ns),
		tag.Insert(r.eventSourceKey, args.eventSource),
		tag.Insert(r.eventTypeKey, args.eventType),
		t)
}
