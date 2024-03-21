/*
Copyright 2020 The Knative Authors

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

package benchmarks

import (
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

// Test Prefix Filter
func BenchmarkPrefixFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			filter, err := subscriptionsapi.NewPrefixFilter(i.(map[string]string))
			if err != nil {
				b.Fatalf("failed to create filter: %v", err)
			}
			return filter
		},
		FilterBenchmark{
			name:   "Pass with prefix match of id",
			arg:    map[string]string{"id": event.ID()[0:5]},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name: "Pass with prefix match of all context attributes",
			arg: map[string]string{
				"id":              event.ID()[0:5],
				"source":          event.Source()[0:5],
				"type":            event.Type()[0:5],
				"dataschema":      event.DataSchema()[0:5],
				"datacontenttype": event.DataContentType()[0:5],
				"subject":         event.Subject()[0:5],
			},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name: "Pass with prefix match of all context attributes",
			arg: map[string]string{
				"id":              event.ID()[0:3],
				"source":          event.Source()[0:3],
				"type":            event.Type()[0:3],
				"dataschema":      event.DataSchema()[0:3],
				"datacontenttype": event.DataContentType()[0:3],
				"subject":         event.Subject()[0:3],
			},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name: "No pass with prefix match of id and source",
			arg: map[string]string{
				"id":     "qwertyuiopasdfghjklzxcvbnm",
				"source": "qwertyuiopasdfghjklzxcvbnm",
			},
			events: []cloudevents.Event{event},
		},
	)
}
