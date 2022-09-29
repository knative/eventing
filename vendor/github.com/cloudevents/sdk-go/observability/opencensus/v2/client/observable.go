/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	// LatencyMs measures the latency in milliseconds for the CloudEvents
	// client methods.
	LatencyMs = stats.Float64("cloudevents.io/sdk-go/client/latency", "The latency in milliseconds for the CloudEvents client methods.", "ms")

	// KeyMethod is the tag used for marking method on a metric.
	KeyMethod, _ = tag.NewKey("method")
	// KeyResult is the tag used for marking result on a metric.
	KeyResult, _ = tag.NewKey("result")

	// LatencyView is an OpenCensus view that shows client method latency.
	LatencyView = &view.View{
		Name:        "client/latency",
		Measure:     LatencyMs,
		Description: "The distribution of latency inside of client for CloudEvents.",
		Aggregation: view.Distribution(0, .01, .1, 1, 10, 100, 1000, 10000),
		TagKeys:     LatencyTags(),
	}
)

func LatencyTags() []tag.Key {
	return []tag.Key{KeyMethod, KeyResult}
}

// Observable represents the the customization used by the Reporter for a given
// measurement and trace for a single method.
type Observable interface {
	MethodName() string
	LatencyMs() *stats.Float64Measure
}

type observed int32

// Adheres to Observable
var _ Observable = observed(0)

const (
	reportSend observed = iota
	reportRequest
	reportReceive
)

// MethodName implements Observable.MethodName
func (o observed) MethodName() string {
	switch o {
	case reportSend:
		return "send"
	case reportRequest:
		return "request"
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
