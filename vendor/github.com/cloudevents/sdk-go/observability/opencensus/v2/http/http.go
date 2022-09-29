/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package http

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"

	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

func roundtripperDecorator(roundTripper http.RoundTripper) http.RoundTripper {
	return &ochttp.Transport{
		Propagation:    &tracecontext.HTTPFormat{},
		Base:           roundTripper,
		FormatSpanName: formatSpanName,
	}
}

func formatSpanName(r *http.Request) string {
	return "cloudevents.http." + r.URL.Path
}

func tracecontextMiddleware(h http.Handler) http.Handler {
	return &ochttp.Handler{
		Propagation:    &tracecontext.HTTPFormat{},
		Handler:        h,
		FormatSpanName: formatSpanName,
	}
}

// NewObservedHTTP creates an HTTP protocol with trace propagating middleware.
func NewObservedHTTP(opts ...cehttp.Option) (*cehttp.Protocol, error) {
	return cehttp.New(append(
		[]cehttp.Option{
			cehttp.WithRoundTripperDecorator(roundtripperDecorator),
			cehttp.WithMiddleware(tracecontextMiddleware),
		},
		opts...,
	)...)
}
