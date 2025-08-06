/*
Copyright 2020 The Knative Authors

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

//nolint:staticcheck  // ST1003:Underscore in package name
package test_images

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/observability"
	eventingotel "knative.dev/eventing/pkg/observability/otel"
	"knative.dev/eventing/pkg/observability/resource"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/observability/tracing"
)

func ParseHeaders(serializedHeaders string) http.Header {
	h := make(http.Header)
	for _, kv := range strings.Split(serializedHeaders, ",") {
		splitted := strings.Split(kv, "=")
		h.Set(splitted[0], splitted[1])
	}
	return h
}

// ParseDurationStr parses `durationStr` as number of seconds (not time.Duration string),
// if parsing fails, returns back default duration.
func ParseDurationStr(durationStr string, defaultDuration int) time.Duration {
	var duration time.Duration
	if d, err := strconv.Atoi(durationStr); err != nil {
		duration = time.Duration(defaultDuration) * time.Second
	} else {
		duration = time.Duration(d) * time.Second
	}
	return duration
}

const (
	ConfigObservabilityEnv = "K_CONFIG_OBSERVABILITY"
	ConfigLoggingEnv       = "K_CONFIG_LOGGING"
)

// ConfigureTracing can be used in test-images to configure tracing.
func ConfigureTracing(ctx context.Context, logger *zap.SugaredLogger, serviceName string) (*tracing.TracerProvider, error) {
	observabilityEnv := os.Getenv(ConfigObservabilityEnv)
	cfg := &observability.Config{}
	err := json.Unmarshal([]byte(observabilityEnv), cfg)
	if err != nil {
		logger.Warn("Error while trying to read the tracing config, using NoopConfig: ", err)
		cfg = observability.DefaultConfig()
	}

	otelResource, err := resource.Default(serviceName)
	if err != nil {
		logger.Warnw("Error while trying to create otelResource, some attributes may not be set", zap.Error(err))
	}

	tracerProvider, err := tracing.NewTracerProvider(
		ctx,
		cfg.Tracing,
		sdktrace.WithResource(otelResource),
	)
	if err != nil {
		tracerProvider = eventingotel.DefaultTraceProvider(ctx, otelResource)
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider, nil
}

// ConfigureLogging can be used in test-images to configure logging.
func ConfigureLogging(ctx context.Context, name string) context.Context {
	loggingEnv := os.Getenv(ConfigLoggingEnv)
	conf, err := logging.JSONToConfig(loggingEnv)
	if err != nil {
		logging.FromContext(ctx).Warn("Error while trying to read the config logging env: ", err)
		return ctx
	}
	l, _ := logging.NewLoggerFromConfig(conf, name)
	return logging.WithLogger(ctx, l)
}
