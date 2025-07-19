/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package http

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

// NewObservedHTTP creates an HTTP protocol with OTel trace propagating middleware.
func NewObservedHTTP(opts ...cehttp.Option) (*cehttp.Protocol, error) {
	// appends the OpenTelemetry Http transport + Middleware wrapper
	// to properly trace outgoing and incoming requests from the client using this protocol
	return cehttp.New(append(
		[]cehttp.Option{
			cehttp.WithRoundTripper(otelhttp.NewTransport(http.DefaultTransport)),
			cehttp.WithMiddleware(func(next http.Handler) http.Handler {
				return otelhttp.NewHandler(next, "cloudevents.http.receiver")
			}),
		},
		opts...,
	)...)
}
