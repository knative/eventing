package json

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	LatencyMs = stats.Float64("datacodec/json/latency", "The latency in milliseconds for the CloudEvents json data codec methods.", "ms")
)

var (
	LatencyView = &view.View{
		Name:        "datacodec/json/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of the json data codec for CloudEvents.",
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
		return "datacodec/json/encode"
	case ReportDecode:
		return "datacodec/json/decode"
	default:
		return "datacodec/json/unknown"
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
