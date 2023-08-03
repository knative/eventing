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
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

func BenchmarkNotFilter(b *testing.B) {
	// Full event with all possible fields filled
	event := cetest.FullEvent()

	filter, _ := subscriptionsapi.NewExactFilter(map[string]string{"id": event.ID()})
	prefixFilter, _ := subscriptionsapi.NewPrefixFilter(map[string]string{"type": "com.github.pull.create"})
	suffixFilter, _ := subscriptionsapi.NewSuffixFilter(map[string]string{"source": "github.com/cloudevents/sdk-go/v2"})

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			return subscriptionsapi.NewNotFilter(i.(eventfilter.Filter))
		},
		FilterBenchmark{
			name:  "Not filter with exact filter test",
			arg:   filter,
			event: event,
		},
		FilterBenchmark{
			name:  "Not filter with prefix filter test",
			arg:   prefixFilter,
			event: event,
		},
		FilterBenchmark{
			name:  "Not filter with suffix filter test",
			arg:   suffixFilter,
			event: event,
		},
	)
}
