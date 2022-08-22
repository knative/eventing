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

package adapter

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"knative.dev/pkg/tracing"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
)

// loggerConfiguratorFromEnvironment configures
// a logger using environment variables.
type loggerConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

// NewLoggerConfiguratorFromEnvironment returns an environment based logger configurator.
func NewLoggerConfiguratorFromEnvironment(env EnvConfigAccessor) LoggerConfigurator {
	return &loggerConfiguratorFromEnvironment{
		env: env,
	}
}

// CreateLogger based on environment variables.
func (c *loggerConfiguratorFromEnvironment) CreateLogger(ctx context.Context) *zap.SugaredLogger {
	return c.env.GetLogger()
}

// metricsExporterConfiguratorFromEnvironment configures
// a metrics exporter using environment variables.
type metricsExporterConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

// NewMetricsExporterConfiguratorFromEnvironment returns an environment based metrics exporter configurator.
func NewMetricsExporterConfiguratorFromEnvironment(env EnvConfigAccessor) MetricsExporterConfigurator {
	return &metricsExporterConfiguratorFromEnvironment{
		env: env,
	}
}

// SetupMetricsExporter based on environment variables.
func (c *metricsExporterConfiguratorFromEnvironment) SetupMetricsExporter(ctx context.Context) {
	logger := logging.FromContext(ctx)
	mc, err := getMetricsConfigFromEnvironment(c.env)
	if err != nil {
		logger.Warn("metrics exporter not configured", zap.Error(err))
		return
	}

	if err = metrics.UpdateExporter(ctx, *mc, logger); err != nil {
		logger.Errorw("failed to create the metrics exporter", zap.Error(err))
	}
}

// tracingConfiguratorFromEnvironment configures tracing using
// environment variables.
type tracingConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

// NewTracingConfiguratorFromEnvironment returns an environment based tracing configurator.
func NewTracingConfiguratorFromEnvironment(env EnvConfigAccessor) TracingConfigurator {
	return &tracingConfiguratorFromEnvironment{
		env: env,
	}
}

// SetupTracing based on environment variables.
func (c *tracingConfiguratorFromEnvironment) SetupTracing(ctx context.Context, _ *TracingConfiguration) tracing.Tracer {
	logger := logging.FromContext(ctx)
	tracer, err := c.env.SetupTracing(logger)
	if err != nil {
		// If tracing doesn't work, we will log an error, but allow the adapter
		// to continue to start.
		logger.Errorw("Error setting up trace publishing", zap.Error(err))
	}
	return tracer
}

// cloudEventsStatusReporterConfiguratorFromEnvironment configures
// the CloudEvents status reporter using environment variables.
type cloudEventsStatusReporterConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

// NewCloudEventsStatusReporterConfiguratorFromEnvironment returns an environment based CloudEvents
// status reporter configurator.
func NewCloudEventsStatusReporterConfiguratorFromEnvironment(env EnvConfigAccessor) CloudEventsStatusReporterConfigurator {
	return &cloudEventsStatusReporterConfiguratorFromEnvironment{
		env: env,
	}
}

// CreateCloudEventsStatusReporter based on environment variables.
// The reporter object is returned.
func (c *cloudEventsStatusReporterConfiguratorFromEnvironment) CreateCloudEventsStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient {
	logger := logging.FromContext(ctx)
	mc, err := getMetricsConfigFromEnvironment(c.env)
	if err != nil {
		logger.Warn("CloudEvents client reporter not configured", zap.Error(err))
		return nil
	}

	return crstatusevent.NewCRStatusEventClient(mc.ConfigMap)
}

// profilerConfiguratorFromEnvironment configures
// the profiler using environment variables.
type profilerConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

// NewProfilerConfiguratorFromEnvironment returns an environment based profiler configurator.
func NewProfilerConfiguratorFromEnvironment(env EnvConfigAccessor) ProfilerConfigurator {
	return &profilerConfiguratorFromEnvironment{
		env: env,
	}
}

// CreateProfilingServer creates and return a profiling server when profiling is enabled,
// nil when it is not.
func (c *profilerConfiguratorFromEnvironment) CreateProfilingServer(ctx context.Context) *http.Server {
	logger := logging.FromContext(ctx)
	mc, err := getMetricsConfigFromEnvironment(c.env)
	if err != nil {
		logger.Warn("profiler not configured", zap.Error(err))
		return nil
	}

	// Configure profiler using environment varibles.
	enabled, err := profiling.ReadProfilingFlag(mc.ConfigMap)
	switch {
	case err != nil:
		logger.Errorw("wrong profiler configuration", zap.Error(err))
		return nil
	case !enabled:
		return nil
	}

	return profiling.NewServer(profiling.NewHandler(logger, true))
}

func getMetricsConfigFromEnvironment(env EnvConfigAccessor) (*metrics.ExporterOptions, error) {
	metricsConfig, err := env.GetMetricsConfig()
	switch {
	case err != nil:
		return nil, fmt.Errorf("failed to process metrics options from environment: %w", err)
	case metricsConfig == nil || metricsConfig.ConfigMap == nil:
		return nil, errors.New("environment metrics options not provided")
	}
	return metricsConfig, nil
}
