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

package broker

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
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

	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII

	// TagResult is a tag key referring to the observed result of an operation.
	TagResult = mustNewTagKey("result")

	// TagFilterResult is a tag key referring to the observed result of a filter
	// operation.
	TagFilterResult = mustNewTagKey("filter_result")

	// TagBroker is a tag key referring to the Broker name serviced by this
	// filter process.
	TagBroker = mustNewTagKey("broker")

	// TagTrigger is a tag key referring to the Trigger name serviced by this
	// filter process.
	TagTrigger = mustNewTagKey("trigger")
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
			Aggregation: view.Distribution(Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100,
			TagKeys:     []tag.Key{TagResult, TagBroker, TagTrigger},
		},
		&view.View{
			Name:        "trigger_filter_time",
			Measure:     MeasureTriggerFilterTime,
			Aggregation: view.Distribution(Buckets125(0.1, 10)...), // 0.1, 0.2, 0.5, 1, 2, 5, 10
			TagKeys:     []tag.Key{TagResult, TagFilterResult, TagBroker, TagTrigger},
		},
	)
	if err != nil {
		panic(err)
	}
}

// mustNewTagKey creates a Tag or panics. This will only fail if the tag key
// doesn't conform to tag name validations.
// TODO OC library should provide this
func mustNewTagKey(k string) tag.Key {
	tagKey, err := tag.NewKey(k)
	if err != nil {
		panic(err)
	}
	return tagKey
}

// Buckets125 generates an array of buckets with approximate powers-of-two
// buckets that also aligns with powers of 10 on every 3rd step. This can
// be used to create a view.Distribution.
func Buckets125(low, high float64) []float64 {
	buckets := []float64{low}
	for last := low; last < high; last = last * 10 {
		buckets = append(buckets, 2*last, 5*last, 10*last)
	}
	return buckets
}
