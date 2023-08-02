/*
Copyright 2023 The Knative Authors

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
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"testing"
)

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
