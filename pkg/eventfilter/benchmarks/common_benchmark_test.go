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
	"context"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
)

type FilterBenchmark struct {
	name  string
	arg   interface{}
	event cloudevents.Event
}

// Avoid DCE
var Filter eventfilter.Filter
var Result eventfilter.FilterResult

// RunFilterBenchmarks executes 2 benchmark runs for each of the provided bench cases:
// 1. "Creation: ..." benchmark measures the time/mem to create the filter, given the filter constructor and the argument
// 2. "Run: ..." benchmark measures the time/mem to execute the filter, given a pre-built filter instance and the provided event
func RunFilterBenchmarks(b *testing.B, filterCtor func(interface{}) eventfilter.Filter, filterBenchmarks ...FilterBenchmark) {
	for _, fb := range filterBenchmarks {
		b.Run("Creation: "+fb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Filter = filterCtor(fb.arg)
			}
		})
		// Filter to use for the run
		f := filterCtor(fb.arg)
		b.Run("Run: "+fb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Result = f.Filter(context.TODO(), fb.event)
			}
		})
	}
}

// Test Exact Filter
func BenchmarkExactFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			filter, err := subscriptionsapi.NewExactFilter(i.(map[string]string))
			if err != nil {
				b.Fatalf("failed to create filter: %v", err)
			}
			return filter
		},
		FilterBenchmark{
			name:  "Pass with exact match of id",
			arg:   map[string]string{"id": event.ID()},
			event: event,
		},
		FilterBenchmark{
			// We don't test time because the exact match on the string apparently doesn't work well,
			// which might cause the filter to fail.
			name: "Pass with exact match of all context attributes (except time)",
			arg: map[string]string{
				"id":              event.ID(),
				"source":          event.Source(),
				"type":            event.Type(),
				"dataschema":      event.DataSchema(),
				"datacontenttype": event.DataContentType(),
				"subject":         event.Subject(),
			},
			event: event,
		},
		FilterBenchmark{
			name: "No pass with exact match of id and source",
			arg: map[string]string{
				"id":     "qwertyuiopasdfghjklzxcvbnm",
				"source": "qwertyuiopasdfghjklzxcvbnm",
			},
			event: event,
		},
	)
}

// Test Any Filter
func BenchmarkAnyFilter(b *testing.B) {
	event := cetest.FullEvent()

	// Define a filter here for testing
	filter1, _ := subscriptionsapi.NewExactFilter(map[string]string{"id": event.ID()})

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			// We create the AnyFilter with our predefined filters.
			return subscriptionsapi.NewAnyFilter(filter1)
		},
		FilterBenchmark{
			name:  "Pass with any match",
			arg:   nil, // In this case, we're not using the arg in filter creation.
			event: event,
		},
	)
}

// Test Not Filter
func BenchmarkNotFilter(b *testing.B) {
	event := cetest.FullEvent()

	filter, _ := subscriptionsapi.NewExactFilter(map[string]string{"id": event.ID()})

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			// Create the NotFilter with our predefined filter.
			return subscriptionsapi.NewNotFilter(filter)
		},
		FilterBenchmark{
			name:  "Not filter test",
			arg:   nil, // In this case, we're not using the arg in filter creation.
			event: event,
		},
	)
}

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
			name:  "Pass with prefix match of id",
			arg:   map[string]string{"id": event.ID()[0:5]},
			event: event,
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
			event: event,
		},
		FilterBenchmark{
			name: "No pass with prefix match of id and source",
			arg: map[string]string{
				"id":     "qwertyuiopasdfghjklzxcvbnm",
				"source": "qwertyuiopasdfghjklzxcvbnm",
			},
			event: event,
		},
	)
}

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
			name: "No pass with suffix match of id and source",
			arg: map[string]string{
				"id":     "qwertyuiopasdfghjklzxcvbnm",
				"source": "qwertyuiopasdfghjklzxcvbnm",
			},
			event: event,
		},
	)
}
