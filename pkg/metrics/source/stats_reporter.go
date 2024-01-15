/*
Copyright 2021 The Knative Authors

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

package source

import (
	"context"

	"go.opencensus.io/stats/view"
	"knative.dev/pkg/metrics"

	eventingmetrics "knative.dev/eventing/pkg/metrics"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

var (
	// eventCountM is a counter which records the number of events sent by the source.
	eventCountM = stats.Int64(
		"event_count",
		"Number of events sent",
		stats.UnitDimensionless,
	)

	// retryEventCountM is a counter which records the number of events sent by the source in retries.
	retryEventCountM = stats.Int64(
		"retry_event_count",
		"Number of retry events sent",
		stats.UnitDimensionless,
	)
	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	namespaceKey           = tag.MustNewKey(eventingmetrics.LabelNamespaceName)
	eventSourceKey         = tag.MustNewKey(eventingmetrics.LabelEventSource)
	eventTypeKey           = tag.MustNewKey(eventingmetrics.LabelEventType)
	eventScheme            = tag.MustNewKey(eventingmetrics.LabelEventScheme)
	sourceNameKey          = tag.MustNewKey(eventingmetrics.LabelName)
	sourceResourceGroupKey = tag.MustNewKey(eventingmetrics.LabelResourceGroup)
	responseCodeKey        = tag.MustNewKey(eventingmetrics.LabelResponseCode)
	responseCodeClassKey   = tag.MustNewKey(eventingmetrics.LabelResponseCodeClass)
	responseError          = tag.MustNewKey(eventingmetrics.LabelResponseError)
	responseTimeout        = tag.MustNewKey(eventingmetrics.LabelResponseTimeout)
)

// ReportArgs defines the arguments for reporting metrics.
type ReportArgs struct {
	Namespace     string
	EventType     string
	EventScheme   string
	EventSource   string
	Name          string
	ResourceGroup string
	Error         string
	Timeout       bool
}

func init() {
	register()
}

// StatsReporter defines the interface for sending source metrics.
type StatsReporter interface {
	// ReportEventCount captures the event count. It records one per call.
	ReportEventCount(args *ReportArgs, responseCode int) error
	ReportRetryEventCount(args *ReportArgs, responseCode int) error
}

var _ StatsReporter = (*reporter)(nil)

// reporter holds cached metric objects to report source metrics.
type reporter struct {
	ctx context.Context
}

// NewStatsReporter creates a reporter that collects and reports source metrics.
func NewStatsReporter() (StatsReporter, error) {
	ctx, err := tag.New(
		context.Background(),
	)
	if err != nil {
		return nil, err
	}
	return &reporter{ctx: ctx}, nil
}

func (r *reporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	ctx, err := r.generateTag(args, responseCode)
	if err != nil {
		return err
	}
	metrics.Record(ctx, eventCountM.M(1))
	return nil
}

func (r *reporter) ReportRetryEventCount(args *ReportArgs, responseCode int) error {
	ctx, err := r.generateTag(args, responseCode)
	if err != nil {
		return err
	}
	metrics.Record(ctx, retryEventCountM.M(1))
	return nil
}

func (r *reporter) generateTag(args *ReportArgs, responseCode int) (context.Context, error) {
	return tag.New(
		r.ctx,
		tag.Insert(namespaceKey, args.Namespace),
		tag.Insert(eventSourceKey, args.EventSource),
		tag.Insert(eventTypeKey, args.EventType),
		tag.Insert(eventScheme, args.EventScheme),
		tag.Insert(sourceNameKey, args.Name),
		tag.Insert(sourceResourceGroupKey, args.ResourceGroup),
		metrics.MaybeInsertIntTag(responseCodeKey, responseCode, responseCode > 0),
		metrics.MaybeInsertStringTag(responseCodeClassKey, metrics.ResponseCodeClass(responseCode), responseCode > 0),
		tag.Insert(responseError, args.Error),
		metrics.MaybeInsertBoolTag(responseTimeout, args.Timeout, args.Error != ""))
}

func register() {
	tagKeys := []tag.Key{
		namespaceKey,
		eventSourceKey,
		eventTypeKey,
		eventScheme,
		sourceNameKey,
		sourceResourceGroupKey,
		responseCodeKey,
		responseCodeClassKey,
		responseError,
		responseTimeout,
	}

	// Create view to see our measurements.
	if err := view.Register(
		&view.View{
			Description: eventCountM.Description(),
			Measure:     eventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
		&view.View{
			Description: retryEventCountM.Description(),
			Measure:     retryEventCountM,
			Aggregation: view.Count(),
			TagKeys:     tagKeys,
		},
	); err != nil {
		panic(err)
	}
}
