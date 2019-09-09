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
	"strconv"

	. "knative.dev/eventing/pkg/metrics/metricskey"
	"knative.dev/eventing/pkg/utils"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/metrics/metricskey"
)

var (
	// eventCountM is a counter which records the number of events sent
	// by the ApiServerSource.
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
	name        string
}

const (
	importerResourceGroupValue = "apiserversources.sources.eventing.knative.dev"
)

// StatsReporter defines the interface for sending filter metrics.
type StatsReporter interface {
	ReportEventCount(args *ReportArgs, responseCode int) error
}

var _ StatsReporter = (*reporter)(nil)

// reporter holds cached metric objects to report filter metrics.
type reporter struct {
	namespaceTagKey             tag.Key
	eventTypeTagKey             tag.Key
	eventSourceTagKey           tag.Key
	importerNameTagKey          tag.Key
	importerResourceGroupTagKey tag.Key
	responseCodeKey             tag.Key
	responseCodeClassKey        tag.Key
}

// NewStatsReporter creates a reporter that collects and reports apiserversource
// metrics.
func NewStatsReporter() (StatsReporter, error) {
	var r = &reporter{}

	// Create the tag keys that will be used to add tags to our measurements.
	nsTag, err := tag.NewKey(metricskey.LabelNamespaceName)
	if err != nil {
		return nil, err
	}
	r.namespaceTagKey = nsTag

	eventSourceTag, err := tag.NewKey(metricskey.LabelEventSource)
	if err != nil {
		return nil, err
	}
	r.eventSourceTagKey = eventSourceTag

	eventTypeTag, err := tag.NewKey(metricskey.LabelEventType)
	if err != nil {
		return nil, err
	}
	r.eventTypeTagKey = eventTypeTag

	importerNameTag, err := tag.NewKey(metricskey.LabelImporterName)
	if err != nil {
		return nil, err
	}
	r.importerNameTagKey = importerNameTag

	importerResourceGroupTag, err := tag.NewKey(metricskey.LabelImporterResourceGroup)
	if err != nil {
		return nil, err
	}
	r.importerResourceGroupTagKey = importerResourceGroupTag

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
			TagKeys:     []tag.Key{r.namespaceTagKey, r.eventSourceTagKey, r.eventTypeTagKey, r.importerNameTagKey, r.importerResourceGroupTagKey, r.responseCodeKey, r.responseCodeClassKey},
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

func (r *reporter) generateTag(args *ReportArgs, responseCode int) (context.Context, error) {
	return tag.New(
		context.Background(),
		tag.Insert(r.namespaceTagKey, args.ns),
		tag.Insert(r.eventSourceTagKey, args.eventSource),
		tag.Insert(r.eventTypeTagKey, args.eventType),
		tag.Insert(r.importerNameTagKey, args.name),
		tag.Insert(r.importerResourceGroupTagKey, importerResourceGroupValue),
		tag.Insert(r.responseCodeKey, strconv.Itoa(responseCode)),
		tag.Insert(r.responseCodeClassKey, utils.ResponseCodeClass(responseCode)))
}
