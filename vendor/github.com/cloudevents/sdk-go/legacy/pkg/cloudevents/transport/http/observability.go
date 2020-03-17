package http

import (
	"fmt"

	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	// LatencyMs measures the latency in milliseconds for the http transport
	// methods for CloudEvents.
	LatencyMs = stats.Float64(
		"cloudevents.io/sdk-go/transport/http/latency",
		"The latency in milliseconds for the http transport methods for CloudEvents.",
		"ms")
)

var (
	// LatencyView is an OpenCensus view that shows http transport method latency.
	LatencyView = &view.View{
		Name:        "transport/http/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of http transport for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	reportSend observed = iota
	reportReceive
	reportServeHTTP
	reportEncode
	reportDecode
)

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportSend:
		return "send"
	case reportReceive:
		return "receive"
	case reportServeHTTP:
		return "servehttp"
	case reportEncode:
		return "encode"
	case reportDecode:
		return "decode"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}

// CodecObserved is a wrapper to append version to observed.
type CodecObserved struct {
	// Method
	o observed
	// Codec
	c string
}

// Adheres to Observable
var _ observability.Observable = (*CodecObserved)(nil)

// MethodName implements Observable.MethodName
func (c CodecObserved) MethodName() string {
	return fmt.Sprintf("%s/%s", c.o.MethodName(), c.c)
}

// LatencyMs implements Observable.LatencyMs
func (c CodecObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
