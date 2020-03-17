package client

import (
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/observability"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

var (
	// LatencyMs measures the latency in milliseconds for the CloudEvents
	// client methods.
	LatencyMs = stats.Float64("cloudevents.io/sdk-go/client/latency", "The latency in milliseconds for the CloudEvents client methods.", "ms")
)

var (
	// LatencyView is an OpenCensus view that shows client method latency.
	LatencyView = &view.View{
		Name:        "client/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of client for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     observability.LatencyTags(),
	}
)

type observed int32

// Adheres to Observable
var _ observability.Observable = observed(0)

const (
	clientSpanName = "cloudevents.client"

	specversionAttr     = "cloudevents.specversion"
	typeAttr            = "cloudevents.type"
	sourceAttr          = "cloudevents.source"
	subjectAttr         = "cloudevents.subject"
	datacontenttypeAttr = "cloudevents.datacontenttype"

	reportSend observed = iota
	reportReceive
)

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportSend:
		return "send"
	case reportReceive:
		return "receive"
	default:
		return "unknown"
	}
}

// LatencyMs implements Observable.LatencyMs
func (o observed) LatencyMs() *stats.Float64Measure {
	return LatencyMs
}

func eventTraceAttributes(e cloudevents.EventContextReader) []trace.Attribute {
	as := []trace.Attribute{
		trace.StringAttribute(specversionAttr, e.GetSpecVersion()),
		trace.StringAttribute(typeAttr, e.GetType()),
		trace.StringAttribute(sourceAttr, e.GetSource()),
	}
	if sub := e.GetSubject(); sub != "" {
		as = append(as, trace.StringAttribute(subjectAttr, sub))
	}
	if dct := e.GetDataContentType(); dct != "" {
		as = append(as, trace.StringAttribute(datacontenttypeAttr, dct))
	}
	return as
}
