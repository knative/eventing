package benchmarks

import (
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
)

func BenchmarkAllFilter(b *testing.B) {
	// Full event with all possible fields filled
	event := cetest.FullEvent()

	filter, _ := subscriptionsapi.NewExactFilter(map[string]string{"id": event.ID()})
	exactFilter2, _ := subscriptionsapi.NewExactFilter(map[string]string{"type": event.Type()})
	exactFilter3, _ := subscriptionsapi.NewExactFilter(map[string]string{"source": event.Source()})

	prefixFilter, _ := subscriptionsapi.NewPrefixFilter(map[string]string{"type": event.Type()[0:5]})
	suffixFilter, _ := subscriptionsapi.NewSuffixFilter(map[string]string{"source": event.Source()[len(event.Source())-5:]})
	prefixFilterNoMatch, _ := subscriptionsapi.NewPrefixFilter(map[string]string{"type": "qwertyuiop"})
	suffixFilterNoMatch, _ := subscriptionsapi.NewSuffixFilter(map[string]string{"source": "qwertyuiop"})

	largeMatchingFilters := make([]eventfilter.Filter, 1000)
	largeNonMatchingFilters := make([]eventfilter.Filter, 1000)

	for i := range largeMatchingFilters {
		largeMatchingFilters[i] = filter
		largeNonMatchingFilters[i] = prefixFilterNoMatch
	}

	alternatingFilters := []eventfilter.Filter{}
	for i := 0; i < 500; i++ {
		alternatingFilters = append(alternatingFilters, filter, prefixFilterNoMatch)
	}

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			filters := i.([]eventfilter.Filter)
			return subscriptionsapi.NewAllFilter(filters...)
		},
		FilterBenchmark{
			name:  "All filter with exact filter test",
			arg:   []eventfilter.Filter{filter},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter match all subfilters",
			arg:   []eventfilter.Filter{filter, prefixFilter, suffixFilter},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter no 1 match at end of array",
			arg:   []eventfilter.Filter{prefixFilterNoMatch, suffixFilterNoMatch, filter},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter no 1 match at start of array",
			arg:   []eventfilter.Filter{filter, prefixFilterNoMatch, suffixFilterNoMatch},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with multiple exact filters that match",
			arg:   []eventfilter.Filter{filter, exactFilter2, exactFilter3},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with one non-matching filter in the middle",
			arg:   []eventfilter.Filter{filter, prefixFilterNoMatch, exactFilter2},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with all non-matching filters",
			arg:   []eventfilter.Filter{prefixFilterNoMatch, suffixFilterNoMatch},
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with large number of sub-filters that match",
			arg:   largeMatchingFilters,
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with large number of sub-filters that do not match",
			arg:   largeNonMatchingFilters,
			event: event,
		},
		FilterBenchmark{
			name:  "All filter with alternating matching and non-matching filters",
			arg:   alternatingFilters,
			event: event,
		},
	)

}
