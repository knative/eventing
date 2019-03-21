package datacodec

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	LatencyMs = stats.Float64("datacodec/latency", "The latency in milliseconds for the CloudEvents generic data codec methods.", "ms")
)

var (
	LatencyView = &view.View{
		Name:        "datacodec/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of the generic data codec for CloudEvents.",
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
		return "datacodec/encode"
	case ReportDecode:
		return "datacodec/decode"
	default:
		return "datacodec/unknown"
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
