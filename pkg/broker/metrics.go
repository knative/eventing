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

const (
	metricsNamespace = "trigger_receiver"
)

var (
	// MeasureMessagesTotal is a counter which records the number of messages
	// received by the trigger. The value of the Result tag indicates whether
	// the message was filtered or dispatched.
	MeasureMessagesTotal = stats.Int64(
		"knative.dev/eventing/trigger/measures/messages_total",
		"Total number of messages received",
		stats.UnitNone,
	)

	// TagTrigger is a tag key referring to the trigger name
	TagTrigger = mustNewTagKey("trigger")

	// TagResult is a tag key referring to the observed result of an operation.
	TagResult = mustNewTagKey("result")
)

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

func initViews() {
	// Create views for exporting measurements.
	err := view.Register(
		&view.View{
			Name:        "messages_total",
			Measure:     MeasureMessagesTotal,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{TagResult, TagTrigger},
		},
	)

	if err != nil {
		panic(err)
	}
}
