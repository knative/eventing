package benchmarks

import (
	"context"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"knative.dev/eventing/pkg/eventfilter"
)

type FilterBenchmark struct {
	name   string
	filter eventfilter.Filter
	event  cloudevents.Event
}

// Avoid DCE
var R eventfilter.FilterResult

func RunFilterBenchmarks(b *testing.B, filterBenchmarks ...FilterBenchmark) {
	for _, fb := range filterBenchmarks {
		b.Run(fb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				R = fb.filter.Filter(context.TODO(), fb.event)
			}
		})
	}
}
