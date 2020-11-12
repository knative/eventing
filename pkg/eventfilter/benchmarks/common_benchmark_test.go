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
var F eventfilter.Filter
var R eventfilter.FilterResult

func RunFilterBenchmarks(b *testing.B, filterCtor func(interface{}) eventfilter.Filter, filterBenchmarks ...FilterBenchmark) {
	for _, fb := range filterBenchmarks {
		b.Run("Creation: "+fb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				F = filterCtor(fb.arg)
			}
		})
		// Filter to use for the run
		f := filterCtor(fb.arg)
		b.Run("Run: "+fb.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				R = f.Filter(context.TODO(), fb.event)
			}
		})
	}
}
