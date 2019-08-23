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
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	utils "knative.dev/eventing/pkg/broker"
)

var (
	// MeasureTriggerEventsTotal is a counter which records the number of events received
	// by a Trigger.
	MeasureTriggerEventsTotal = stats.Int64(
		"knative.dev/eventing/trigger/measures/events_total",
		"Total number of events received by a Trigger",
		stats.UnitNone,
	)

	// MeasureTriggerDispatchTime records the time spent dispatching an event for
	// a Trigger, in milliseconds.
	MeasureTriggerDispatchTime = stats.Int64(
		"knative.dev/eventing/trigger/measures/dispatch_time",
		"Time spent dispatching an event to a Trigger",
		stats.UnitMilliseconds,
	)

	// MeasureTriggerFilterTime records the time spent filtering a message for a
	// Trigger, in milliseconds.
	MeasureTriggerFilterTime = stats.Int64(
		"knative.dev/eventing/trigger/measures/filter_time",
		"Time spent filtering a message for a Trigger",
		stats.UnitMilliseconds,
	)

	// MeasureDeliveryTime records the time spent between arrival at ingress
	// and delivery to the trigger subscriber.
	MeasureDeliveryTime = stats.Int64(
		"knative.dev/eventing/trigger/measures/delivery_time",
		"Time between an event arriving at ingress and delivery to the trigger subscriber",
		stats.UnitMilliseconds,
	)

	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII

	// TagResult is a tag key referring to the observed result of an operation.
	TagResult = utils.MustNewTagKey("result")

	// TagFilterResult is a tag key referring to the observed result of a filter
	// operation.
	TagFilterResult = utils.MustNewTagKey("filter_result")

	// TagBroker is a tag key referring to the Broker name serviced by this
	// filter process.
	TagBroker = utils.MustNewTagKey("broker")

	// TagTrigger is a tag key referring to the Trigger name serviced by this
	// filter process.
	TagTrigger = utils.MustNewTagKey("trigger")
)

func init() {
	// Create views for exporting measurements. This returns an error if a
	// previously registered view has the same name with a different value.
	err := view.Register(
		&view.View{
			Name:        "trigger_events_total",
			Measure:     MeasureTriggerEventsTotal,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{TagResult, TagBroker, TagTrigger},
		},
		&view.View{
			Name:        "trigger_dispatch_time",
			Measure:     MeasureTriggerDispatchTime,
			Aggregation: view.Distribution(utils.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100,
			TagKeys:     []tag.Key{TagResult, TagBroker, TagTrigger},
		},
		&view.View{
			Name:        "trigger_filter_time",
			Measure:     MeasureTriggerFilterTime,
			Aggregation: view.Distribution(utils.Buckets125(0.1, 10)...), // 0.1, 0.2, 0.5, 1, 2, 5, 10
			TagKeys:     []tag.Key{TagResult, TagFilterResult, TagBroker, TagTrigger},
		},
		&view.View{
			Name:        "broker_to_function_delivery_time",
			Measure:     MeasureDeliveryTime,
			Aggregation: view.Distribution(utils.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100
			TagKeys:     []tag.Key{TagResult, TagBroker, TagTrigger},
		},
	)
	if err != nil {
		panic(err)
	}
}
