package xml

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	LatencyMs = stats.Float64("datacodec/xml/latency", "The latency in milliseconds for the CloudEvents xml data codec methods.", "ms")
)

var (
	LatencyView = &view.View{
		Name:        "datacodec/xml/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of the xml data codec for CloudEvents.",
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
		return "datacodec/xml/encode"
	case ReportDecode:
		return "datacodec/xml/decode"
	default:
		return "datacodec/xml/unknown"
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
