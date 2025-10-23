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

package eventshub

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	otelnoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"knative.dev/pkg/changeset"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/observability"
	"knative.dev/pkg/observability/metrics"
	"knative.dev/pkg/observability/tracing"
	"knative.dev/pkg/system"
)

const (
	// Deprecated: use ConfigObservabilityEnv instead
	ConfigTracingEnv = "K_CONFIG_TRACING"

	ConfigLoggingEnv       = "K_CONFIG_LOGGING"
	ConfigObservabilityEnv = "K_CONFIG_OBSERVABILITY"
	EventGeneratorsEnv     = "EVENT_GENERATORS"
	EventLogsEnv           = "EVENT_LOGS"

	OIDCEnabledEnv                         = "ENABLE_OIDC_AUTH"
	OIDCGenerateExpiredTokenEnv            = "OIDC_GENERATE_EXPIRED_TOKEN"
	OIDCGenerateInvalidAudienceTokenEnv    = "OIDC_GENERATE_INVALID_AUDIENCE_TOKEN"
	OIDCSubjectEnv                         = "OIDC_SUBJECT"
	OIDCGenerateCorruptedSignatureTokenEnv = "OIDC_GENERATE_CORRUPTED_SIG_TOKEN"
	OIDCSinkAudienceEnv                    = "OIDC_SINK_AUDIENCE"
	OIDCReceiverAudienceEnv                = "OIDC_AUDIENCE"
	OIDCTokenEnv                           = "OIDC_TOKEN"

	EnforceTLS    = "ENFORCE_TLS"
	tlsIssuerKind = "TLS_ISSUER_KIND"
	tlsIssuerName = "TLS_ISSUER_NAME"

	otelServiceNameKey = "OTEL_SERVICE_NAME"
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

// Deprecated: use ConfigureObservability instead. This will now always return a noop tracer
// ConfigureTracing can be used in test-images to configure tracing
func ConfigureTracing(logger *zap.SugaredLogger, serviceName string) (trace.Tracer, error) {
	return otelnoop.NewTracerProvider().Tracer(serviceName), nil
}

func ConfigureObservability(ctx context.Context, logger *zap.SugaredLogger, serviceName string) (*metrics.MeterProvider, *tracing.TracerProvider, error) {
	obsCfgEnv := os.Getenv(ConfigObservabilityEnv)

	cfg, err := ParseObservabilityConfig(obsCfgEnv)
	if err != nil {
		logger.Warnw("Error while trying to parse observability config from env, falling back to default config", zap.Error(err))
	}

	cfg = NewObservabilityConfigFromExistingWithDefaults(cfg)

	resource, err := defaultResource(serviceName)
	if err != nil {
		logger.Warnw("Error while creating otel resource, resource may be missing some attributes", zap.Error(err))
	}

	meterProvider, err := metrics.NewMeterProvider(
		ctx,
		cfg.Metrics,
		sdkmetric.WithResource(resource),
	)
	if err != nil {
		return nil, nil, err
	}

	otel.SetMeterProvider(meterProvider)

	tracerProvider, err := tracing.NewTracerProvider(
		ctx,
		cfg.Tracing,
		sdktrace.WithResource(resource),
	)
	if err != nil {
		return nil, nil, err
	}

	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())
	otel.SetTracerProvider(tracerProvider)

	return meterProvider, tracerProvider, nil
}

func defaultResource(serviceName string) (*resource.Resource, error) {
	// If the OTEL_SERVICE_NAME is set then let this override
	// our own serviceName
	if name := os.Getenv(otelServiceNameKey); name != "" {
		serviceName = name
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceVersion(changeset.Get()),
		semconv.ServiceName(serviceName),
	}

	if ns := os.Getenv("SYSTEM_NAMESPACE"); ns != "" {
		attrs = append(attrs, semconv.K8SNamespaceName(ns))
	}

	if pn := system.PodName(); pn != "" {
		attrs = append(attrs, semconv.K8SPodName(pn))
	}

	// Ignore the error because it complains about semconv
	// schema version differences
	resource, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			attrs...,
		),
	)
	return resource, err
}

func ParseObservabilityConfig(configStr string) (*observability.Config, error) {
	obsCfg := &observability.Config{}
	err := json.Unmarshal([]byte(configStr), obsCfg)
	return obsCfg, err
}

func NewObservabilityConfigFromExistingWithDefaults(cfg *observability.Config) *observability.Config {
	defaultCfg := observability.DefaultConfig()
	if cfg == nil {
		return cfg
	}

	newCfg := &observability.Config{
		Metrics: cfg.Metrics,
		Runtime: cfg.Runtime,
		Tracing: cfg.Tracing,
	}
	var emptyMetrics observability.MetricsConfig
	if newCfg.Metrics == emptyMetrics {
		newCfg.Metrics = defaultCfg.Metrics
	}
	var emptyRuntime observability.RuntimeConfig
	if newCfg.Runtime == emptyRuntime {
		newCfg.Runtime = defaultCfg.Runtime
	}
	var emptyTracing observability.TracingConfig
	if newCfg.Tracing == emptyTracing {
		newCfg.Tracing = defaultCfg.Tracing
	}

	return newCfg
}

// ConfigureTracing can be used in test-images to configure tracing
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

// WithServerTracing wraps the provided handler in a tracing handler
func WithServerTracing(handler http.Handler) http.Handler {
	return otelhttp.NewHandler(
		handler,
		"receive",
		otelhttp.WithPropagators(tracing.DefaultTextMapPropagator()),
	)
}

// WithClientTracing enables exporting traces by the client's transport.
func WithClientTracing(client *http.Client) error {
	prev := client.Transport
	client.Transport = otelhttp.NewTransport(
		prev,
		otelhttp.WithPropagators(tracing.DefaultTextMapPropagator()),
	)
	return nil
}

type HandlerFunc func(handler http.Handler) http.Handler
type ClientOption func(*http.Client) error
