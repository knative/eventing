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

package main

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

const (
	metricsNamespace = "broker_ingress"
)

var (
	// MeasureMessagesTotal is a counter which records the number of messages received
	// by the ingress. The value of the Result tag indicates whether the message
	// was filtered or dispatched.
	MeasureMessagesTotal = stats.Int64(
		"knative.dev/eventing/broker/ingress/measures/messages_total",
		"Total number of messages received",
		stats.UnitNone,
	)

	// MeasureDispatchTime records the time spent dispatching a message, in milliseconds.
	MeasureDispatchTime = stats.Int64(
		"knative.dev/eventing/broker/ingress/measures/dispatch_time",
		"Time spent dispatching a message",
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
			Name:        "messages_total",
			Measure:     MeasureMessagesTotal,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{TagResult, TagBroker},
		},
		&view.View{
			Name:        "dispatch_time",
			Measure:     MeasureDispatchTime,
			Aggregation: view.Distribution(10, 100, 1000, 10000),
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
