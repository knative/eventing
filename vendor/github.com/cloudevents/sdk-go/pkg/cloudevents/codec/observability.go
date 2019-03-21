package codec

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	LatencyMs = stats.Float64("codec/json/latency", "The latency in milliseconds for the CloudEvents json codec methods.", "ms")
)

var (
	LatencyView = &view.View{
		Name:        "codec/json/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of the json codec for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type Observed int32

const (
	ReportEncode Observed = iota
	ReportDecode
)

func (o Observed) TraceName() string {
	switch o {
	case ReportEncode:
		return "codec/json/encode"
	case ReportDecode:
		return "codec/json/decode"
	default:
		return "codec/unknown"
	}
}

func (o Observed) MethodName() string {
	switch o {
	case ReportEncode:
		return "encode"
	case ReportDecode:
		return "decode"
	default:
		return "unknown"
	}
}

func (o Observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}

// CodecObserved is a wrapper to append version to Observed.
type CodecObserved struct {
	// Method
	o Observed
	// Version
	v string
}

func (c CodecObserved) TraceName() string {
	return fmt.Sprintf("%s/%s", c.o.TraceName(), c.v)
}

func (c CodecObserved) MethodName() string {
	return fmt.Sprintf("%s/%s", c.o.MethodName(), c.v)
}

func (c CodecObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
