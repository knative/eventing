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
	"github.com/knative/eventing/pkg/broker"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	// MeasureEventsTotal is a counter which records the number of events received
	// by the ingress. The value of the Result tag indicates whether the event
	// was filtered or dispatched by the ingress.
	MeasureEventsTotal = stats.Int64(
		"knative.dev/eventing/broker/measures/events_total",
		"Total number of events received",
		stats.UnitNone,
	)

	// MeasureDispatchTime records the time spent dispatching an event, in
	// milliseconds.
	MeasureDispatchTime = stats.Int64(
		"knative.dev/eventing/broker/measures/dispatch_time",
		"Time spent dispatching an event",
		stats.UnitMilliseconds,
	)

	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII

	// TagResult is a tag key referring to the observed result of an operation.
	TagResult = mustNewTagKey("result")

	// TagBroker is a tag key referring to the Broker name serviced by this
	// ingress process.
	TagBroker = mustNewTagKey("broker")
)

func init() {
	// Create views for exporting measurements. This returns an error if a
	// previously registered view has the same name with a different value.
	err := view.Register(
		&view.View{
			Name:        "broker_events_total",
			Measure:     MeasureEventsTotal,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{TagResult, TagBroker},
		},
		&view.View{
			Name:        "broker_dispatch_time",
			Measure:     MeasureDispatchTime,
			Aggregation: view.Distribution(broker.Buckets125(1, 100)...), // 1, 2, 5, 10, 20, 50, 100
			TagKeys:     []tag.Key{TagResult, TagBroker},
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
