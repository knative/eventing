/*
Copyright 2025 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package otel

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func HandlerOptions(meterProvider metric.MeterProvider, traceProvider trace.TracerProvider) []otelhttp.Option {
	return []otelhttp.Option{
		otelhttp.WithMeterProvider(meterProvider),
		otelhttp.WithTracerProvider(traceProvider),
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			if r.URL.Path == "" {
				return r.Method + " /"
			}
			return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		}),
	}
}
