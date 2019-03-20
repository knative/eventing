package http

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

var (
	LatencyMs = stats.Float64(
		"transport/http/latency",
		"The latency in milliseconds for the http transport methods for CloudEvents.",
		"ms")
)

var (
	LatencyView = &view.View{
		Name:        "transport/http/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of http transport for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type Observed int32

const (
	ReportSend Observed = iota
	ReportReceive
	ReportServeHTTP
	ReportEncode
	ReportDecode
)

func (o Observed) TraceName() string {
	switch o {
	case ReportSend:
		return "transport/http/send"
	case ReportReceive:
		return "transport/http/receive"
	case ReportServeHTTP:
		return "transport/http/servehttp"
	case ReportEncode:
		return "transport/http/encode"
	case ReportDecode:
		return "transport/http/decode"
	default:
		return "transport/http/unknown"
	}
}

func (o Observed) MethodName() string {
	switch o {
	case ReportSend:
		return "send"
	case ReportReceive:
		return "receive"
	case ReportServeHTTP:
		return "servehttp"
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
	// Codec
	c string
}

func (c CodecObserved) TraceName() string {
	return fmt.Sprintf("%s/%s", c.o.TraceName(), c.c)
}

func (c CodecObserved) MethodName() string {
	return fmt.Sprintf("%s/%s", c.o.MethodName(), c.c)
}

func (c CodecObserved) LatencyMs() *stats.Float64Measure {
	return c.o.LatencyMs()
}
