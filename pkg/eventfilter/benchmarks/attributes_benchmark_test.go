package benchmarks

import (
	"testing"

	cetest "github.com/cloudevents/sdk-go/v2/test"

	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/attributes"
)

func BenchmarkAttributesFilter(b *testing.B) {
	event := cetest.FullEvent()

	RunFilterBenchmarks(b,
		func(i interface{}) eventfilter.Filter {
			return attributes.NewAttributesFilter(i.(map[string]string))
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
