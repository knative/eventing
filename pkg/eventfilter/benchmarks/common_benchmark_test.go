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
