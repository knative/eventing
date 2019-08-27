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
	utils "knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/pkg/metrics"
)

var (
	// eventCountResource is a counter which records the number of events for
	// resource mode.
	eventCountResource = stats.Int64(
		"event_count_resource_mode",
		"Number of events received in resource mode",
		stats.UnitDimensionless,
	)

	// eventCountResource is a counter which records the number of events for
	// reference mode.
	eventCountRef = stats.Int64(
		"event_count_resource_mode",
		"Number of events received in reference mode",
		stats.UnitDimensionless,
	)
)

type ReportArgs struct {
	ns          string
	eventType   string
	eventSource string
}

// FilterResult has the result of the filtering operation.
type FilterResult string

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCountResource(args *ReportArgs, err error) error
	ReportEventCountRef(args *ReportArgs, err error) error
}

// reporter holds cached metric objects to report filter metrics.
type reporter struct {
	namespaceTagKey tag.Key
	resultKey       tag.Key
	filterResultKey tag.Key
}

var _ StatsReporter = (*reporter)(nil)

// NewStatsReporter creates a reporter that collects and reports filter metrics.
func NewStatsReporter() (StatsReporter, error) {
	var r = &reporter{}

	// Create the tag keys that will be used to add tags to our measurements.
	nsTag, err := tag.NewKey(metricskey.NamespaceName)
	if err != nil {
		return nil, err
	}
	r.namespaceTagKey = nsTag
	filterResultTag, err := tag.NewKey(metricskey.FilterResult)
	if err != nil {
		return nil, err
	}
	r.filterResultKey = filterResultTag
	resultTag, err := tag.NewKey(metricskey.Result)
	if err != nil {
		return nil, err
	}
	r.resultKey = resultTag

	// Create view to see our measurements.
	err = view.Register(
		&view.View{
			Description: eventCountResource.Description(),
			Measure:     eventCountResource,
			// TODO count or sum aggregation?
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.namespaceTagKey, r.resultKey},
		},
		&view.View{
			Description: eventCountResource.Description(),
			Measure:     eventCountResource,
			// TODO count or sum aggregation?
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{r.namespaceTagKey, r.resultKey},
		},
	)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// ReportEventCount captures the event count.
func (r *reporter) ReportEventCountResource(args *ReportArgs, err error) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, utils.Result(err)))
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountResource.M(1))
	return nil
}

// ReportDispatchTime captures dispatch times.
func (r *reporter) ReportEventCountRef(args *ReportArgs, err error) error {
	ctx, err := r.generateTag(args, tag.Insert(r.resultKey, utils.Result(err)))
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountResource.M(1))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, t tag.Mutator) (context.Context, error) {
	// Note that eventType and eventSource can be empty strings, so they need a special treatment.
	return tag.New(
		context.Background(),
		tag.Insert(r.namespaceTagKey, args.ns),
		t)
}

func valueOrAny(v string) string {
	if v != "" {
		return v
	}
	return metricskey.Any
}
