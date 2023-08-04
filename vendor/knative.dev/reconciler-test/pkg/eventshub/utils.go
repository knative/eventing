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
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracing"
	"knative.dev/pkg/tracing/config"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

const (
	ConfigTracingEnv   = "K_CONFIG_TRACING"
	ConfigLoggingEnv   = "K_CONFIG_LOGGING"
	EventGeneratorsEnv = "EVENT_GENERATORS"
	EventLogsEnv       = "EVENT_LOGS"
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

// ConfigureTracing can be used in test-images to configure tracing
func ConfigureTracing(logger *zap.SugaredLogger, serviceName string) (tracing.Tracer, error) {
	tracingEnv := os.Getenv(ConfigTracingEnv)

	var (
		tracer tracing.Tracer
		err    error
	)

	if tracingEnv == "" {
		tracer, err = tracing.SetupPublishingWithStaticConfig(logger, serviceName, config.NoopConfig())
		if err != nil {
			return tracer, err
		}
	}

	conf, err := config.JSONToTracingConfig(tracingEnv)
	if err != nil {
		return tracer, err
	}

	tracer, err = tracing.SetupPublishingWithStaticConfig(logger, serviceName, conf)
	if err != nil {
		return tracer, err
	}

	return tracer, nil
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
	return &ochttp.Handler{
		Propagation: tracecontextb3.TraceContextEgress,
		Handler:     handler,
	}
}

// WithClientTracing enables exporting traces by the client's transport.
func WithClientTracing(client *http.Client) error {
	client.Transport = &ochttp.Transport{
		Base:        http.DefaultTransport,
		Propagation: tracecontextb3.TraceContextEgress,
	}
	return nil
}

type HandlerFunc func(handler http.Handler) http.Handler
type ClientOption func(*http.Client) error
