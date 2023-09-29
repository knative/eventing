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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

func BenchmarkAnyFilter(b *testing.B) {
	// Full event with all possible fields filled
	event := cetest.FullEvent()
	otherEvent := cetest.FullEvent()
	otherEvent.SetType("qwertyuiop")

	filter, _ := subscriptionsapi.NewExactFilter(map[string]string{"id": event.ID()})
	prefixFilter, _ := subscriptionsapi.NewPrefixFilter(map[string]string{"type": event.Type()[0:5]})
	suffixFilter, _ := subscriptionsapi.NewSuffixFilter(map[string]string{"source": event.Source()[len(event.Source())-5:]})
	prefixFilterNoMatch, _ := subscriptionsapi.NewPrefixFilter(map[string]string{"type": "qwertyuiop"})
	suffixFilterNoMatch, _ := subscriptionsapi.NewSuffixFilter(map[string]string{"source": "qwertyuiop"})

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			filters := i.([]eventfilter.Filter)
			return subscriptionsapi.NewAnyFilter(filters...)
		},
		FilterBenchmark{
			name:   "Any filter with exact filter test",
			arg:    []eventfilter.Filter{filter},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name:   "Any filter match all subfilters",
			arg:    []eventfilter.Filter{filter, prefixFilter, suffixFilter},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name:   "Any filter no 1 match at end of array",
			arg:    []eventfilter.Filter{prefixFilterNoMatch, suffixFilterNoMatch, filter},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name:   "Any filter no 1 match at start of array",
			arg:    []eventfilter.Filter{filter, prefixFilterNoMatch, suffixFilterNoMatch},
			events: []cloudevents.Event{event},
		},
		FilterBenchmark{
			name:   "Any filter 2 events match 2 different filters",
			arg:    []eventfilter.Filter{prefixFilter, prefixFilterNoMatch},
			events: []cloudevents.Event{event, otherEvent},
		},
		FilterBenchmark{
			name:   "Any filter 2 events match 2 different filters, one filter in front which matches neither",
			arg:    []eventfilter.Filter{suffixFilterNoMatch, prefixFilter, prefixFilterNoMatch},
			events: []cloudevents.Event{event, otherEvent},
		},
	)
}
