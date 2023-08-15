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

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

// Test Suffix Filter
func BenchmarkSuffixFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			filter, err := subscriptionsapi.NewSuffixFilter(i.(map[string]string))
			if err != nil {
				b.Fatalf("failed to create filter: %v", err)
			}
			return filter
		},
		FilterBenchmark{
			name:  "Pass with suffix match of id",
			arg:   map[string]string{"id": event.ID()[len(event.ID())-5:]},
			event: event,
		},
		FilterBenchmark{
			name: "Pass with suffix match of all context attributes",
			arg: map[string]string{
				"id":              event.ID()[len(event.ID())-5:],
				"source":          event.Source()[len(event.Source())-5:],
				"type":            event.Type()[len(event.Type())-5:],
				"dataschema":      event.DataSchema()[len(event.DataSchema())-5:],
				"datacontenttype": event.DataContentType()[len(event.DataContentType())-5:],
				"subject":         event.Subject()[len(event.Subject())-5:],
			},
			event: event,
		},
		FilterBenchmark{
			name: "Pass with suffix match of all context attributes",
			arg: map[string]string{
				"id":              event.ID()[len(event.ID())-3:],
				"source":          event.Source()[len(event.Source())-3:],
				"type":            event.Type()[len(event.Type())-3:],
				"dataschema":      event.DataSchema()[len(event.DataSchema())-3:],
				"datacontenttype": event.DataContentType()[len(event.DataContentType())-3:],
				"subject":         event.Subject()[len(event.Subject())-3:],
			},
			event: event,
		},
		FilterBenchmark{
			name: "No pass with suffix match of id and source",
			arg: map[string]string{
				"id":     "qwertyuiopasdfghjklzxcvbnm",
				"source": "qwertyuiopasdfghjklzxcvbnm",
			},
			event: event,
		},
	)
}
