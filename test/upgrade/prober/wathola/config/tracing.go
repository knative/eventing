/*
Copyright 2022 The Knative Authors

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

package config

import (
	"context"
	"encoding/json"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/observability"
	eventingotel "knative.dev/eventing/pkg/observability/otel"
	"knative.dev/eventing/pkg/observability/resource"
	"knative.dev/pkg/observability/tracing"
)

var tracer *tracing.TracerProvider

func SetupTracing() {
	cfg := &observability.Config{}
	err := json.Unmarshal([]byte(Instance.ObservabilityConfig), cfg)
	if err != nil {
		Log.Warn("Tracing configuration is invalid, using the no-op default", zap.Error(err))
	}

	otelResource, err := resource.Default("wathola_prober")
	if err != nil {
		Log.Warnw("Error while trying to create otelResource, some attributes may not be set", zap.Error(err))
	}

	tracerProvider, err := tracing.NewTracerProvider(
		context.Background(),
		cfg.Tracing,
		sdktrace.WithResource(otelResource),
	)
	if err != nil {
		tracerProvider = eventingotel.DefaultTraceProvider(context.Background(), otelResource)
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)
}

func ShutdownTracing() {
	if tracer != nil {
		if err := tracer.Shutdown(context.Background()); err != nil {
			Log.Warn("Failed to shutdown tracing")
		}
	}
}
