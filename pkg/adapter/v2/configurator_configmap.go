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
	"fmt"
	"net/http"
	"os"

	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/profiling"
	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
)

const (
	defaultMetricsPort   = 9092
	defaultMetricsDomain = "knative.dev/eventing"
)

// loggerConfiguratorFromConfigMap dynamically
// configures a logger using a watcher on a Configmap.
type loggerConfiguratorFromConfigMap struct {
	component     string
	configMapName string
}

// LoggerConfiguratorFromConfigMapOption for teawking the logger configurator.
type LoggerConfiguratorFromConfigMapOption func(*loggerConfiguratorFromConfigMap)

// WithLoggerConfiguratorConfigMapName sets the ConfigMap name for the logger configuration.
func WithLoggerConfiguratorConfigMapName(name string) LoggerConfiguratorFromConfigMapOption {
	return func(c *loggerConfiguratorFromConfigMap) {
		c.configMapName = name
	}
}

// NewLoggerConfiguratorFromConfigMap returns a ConfigMap based logger configurator.
func NewLoggerConfiguratorFromConfigMap(component string, opts ...LoggerConfiguratorFromConfigMapOption) LoggerConfigurator {
	c := &loggerConfiguratorFromConfigMap{
		component:     component,
		configMapName: logging.ConfigMapName(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// CreateLogger based on the component's ConfigMap.
// A Watcher is set to update the configuration dynamically.
func (c *loggerConfiguratorFromConfigMap) CreateLogger(ctx context.Context) *zap.SugaredLogger {
	// Use any pre-existing logger to inform
	logger := logging.FromContext(ctx)

	// Get logging configuration from ConfigMap or a
	// default configuration if the ConfigMap was not found.
	lcm, err := GetConfigMapByPolling(ctx, c.configMapName)
	switch {
	case err != nil:
		logger.Errorw("logging ConfigMap "+c.configMapName+
			" could not be retrieved, falling back to defaults", zap.Error(err))
	case lcm == nil:
		logger.Warn("logging configuration not found, falling back to defaults")
	}

	var lc *logging.Config
	if lcm == nil {
		lc, err = logging.NewConfigFromMap(nil)
	} else {
		lc, err = logging.NewConfigFromConfigMap(lcm)
	}

	// Not being able to create the logging configuration is not expected, in
	// such case panic.
	if err != nil {
		logger.Fatal("could not build the logging configuration", zap.Error(err))
	}

	logger, atomicLevel := SetupLoggerFromConfig(lc, c.component)

	logger.Infof("Adding Watcher on ConfigMap %s for logs", c.configMapName)

	cmw := ConfigWatcherFromContext(ctx)
	cmw.Watch(c.configMapName, logging.UpdateLevelFromConfigMap(logger, atomicLevel, c.component))

	return logger
}

// SetupLoggerFromConfig sets up the logger using the provided config
// and returns a logger and atomic level, or dies by calling log.Fatalf.
func SetupLoggerFromConfig(config *logging.Config, component string) (*zap.SugaredLogger, zap.AtomicLevel) {
	l, level := logging.NewLoggerFromConfig(config, component)

	// If PodName is injected into the env vars, set it on the logger.
	if pn := os.Getenv("POD_NAME"); pn != "" {
		l = l.With(zap.String(logkey.Pod, pn))
	}

	return l, level
}

// metricsExporterConfiguratorFromConfigMap dynamically configures
// a metrics exporter using a watcher on a Configmap.
type metricsExporterConfiguratorFromConfigMap struct {
	component     string
	configMapName string
	metricsDomain string
	metricsPort   int
}

// MetricsExporterConfiguratorFromConfigMapOption for teawking the metrics exporter configurator.
type MetricsExporterConfiguratorFromConfigMapOption func(*metricsExporterConfiguratorFromConfigMap)

// WithMetricsExporterConfiguratorConfigMapName sets the ConfigMap name for the metrics exporter configuration.
func WithMetricsExporterConfiguratorConfigMapName(name string) MetricsExporterConfiguratorFromConfigMapOption {
	return func(c *metricsExporterConfiguratorFromConfigMap) {
		c.configMapName = name
	}
}

// WithMetricsExporterConfiguratorMetricsDomain sets the metrics domain for the metrics exporter configuration.
func WithMetricsExporterConfiguratorMetricsDomain(domain string) MetricsExporterConfiguratorFromConfigMapOption {
	return func(c *metricsExporterConfiguratorFromConfigMap) {
		c.metricsDomain = domain
	}
}

// WithMetricsExporterConfiguratorMetricsPort sets the metrics exporter port for the metrics exporter configuration.
func WithMetricsExporterConfiguratorMetricsPort(port int) MetricsExporterConfiguratorFromConfigMapOption {
	return func(c *metricsExporterConfiguratorFromConfigMap) {
		c.metricsPort = port
	}
}

// NewMetricsExporterConfiguratorFromConfigMap returns a ConfigMap based metrics exporter configurator.
func NewMetricsExporterConfiguratorFromConfigMap(component string, opts ...MetricsExporterConfiguratorFromConfigMapOption) MetricsExporterConfigurator {
	c := &metricsExporterConfiguratorFromConfigMap{
		component:     component,
		configMapName: metrics.ConfigMapName(),
		metricsPort:   defaultMetricsPort,
	}

	// metricDomainDefaulter is an AdapterDynamicconfig option that
	// retrieves the adater's metric domain by looking at the
	// environments metrics.DomainEnv variable first,
	// then to the component's hardcoded default metrics domain,
	// and if not present to the eventing default metrics domain.
	if os.Getenv(metrics.DomainEnv) != "" {
		c.metricsDomain = os.Getenv(metrics.DomainEnv)
	} else {
		c.metricsDomain = defaultMetricsDomain
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetupMetricsExporter based on the component's ConfigMap.
// A Watcher is set to update the configuration dynamically.
func (c *metricsExporterConfiguratorFromConfigMap) SetupMetricsExporter(ctx context.Context) {
	logger := logging.FromContext(ctx)

	// Configure watcher to update metrics from ConfigMap.
	updateMetricsFunc, err := metrics.UpdateExporterFromConfigMapWithOpts(ctx, metrics.ExporterOptions{
		Domain:         c.metricsDomain,
		Component:      c.component,
		PrometheusPort: c.metricsPort,
		Secrets:        SecretFetcher(ctx),
	}, logger)
	if err != nil {
		logger.Fatal("Failed to create metrics exporter update function", zap.Error(err))
	}

	logger.Infof("Adding Watcher on ConfigMap %s for metrics", c.configMapName)
	ConfigWatcherFromContext(ctx).Watch(c.configMapName, updateMetricsFunc)
}

// tracingConfiguratorFromConfigMap dynamically
// configures tracing using a watcher on a Configmap.
type tracingConfiguratorFromConfigMap struct {
	configMapName string
}

// TracingConfiguratorFromConfigMapOption for teawking the tracing configurator.
type TracingConfiguratorFromConfigMapOption func(*tracingConfiguratorFromConfigMap)

// WithTracingConfiguratorConfigMapName sets the ConfigMap name for the tracing configuration.
func WithTracingConfiguratorConfigMapName(name string) TracingConfiguratorFromConfigMapOption {
	return func(c *tracingConfiguratorFromConfigMap) {
		c.configMapName = name
	}
}

// NewTracingConfiguratorFromConfigMap returns a ConfigMap based tracing configurator.
func NewTracingConfiguratorFromConfigMap(opts ...TracingConfiguratorFromConfigMapOption) TracingConfigurator {
	c := &tracingConfiguratorFromConfigMap{
		configMapName: tracingconfig.ConfigName,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// SetupTracing based on the component's ConfigMap.
// A Watcher is set to update the configuration dynamically.
func (c *tracingConfiguratorFromConfigMap) SetupTracing(ctx context.Context, cfg *TracingConfiguration) tracing.Tracer {
	logger := logging.FromContext(ctx)

	cmw := ConfigWatcherFromContext(ctx)
	service := fmt.Sprintf("%s.%s", cfg.InstanceName, NamespaceFromContext(ctx))

	logger.Infof("Adding Watcher on ConfigMap %s for tracing", c.configMapName)
	tracer, err := tracing.SetupPublishingWithDynamicConfig(logger, cmw, service, c.configMapName)
	if err != nil {
		logger.Errorw("Error setting up trace publishing. Tracing configuration will be ignored.", zap.Error(err))
	}
	return tracer
}

// cloudEventsStatusReporterConfiguratorFromConfigMap dynamically configures
// the CloudEvents status reporter using a watcher on a Configmap.
type cloudEventsStatusReporterConfiguratorFromConfigMap struct {
	configMapName string
}

// CloudEventsStatusReporterConfiguratorFromConfigMapOption for teawking the CloudEvents
// status reporter configurator.
type CloudEventsStatusReporterConfiguratorFromConfigMapOption func(*cloudEventsStatusReporterConfiguratorFromConfigMap)

// WithCloudEventsStatusReporterConfiguratorConfigMapName sets the ConfigMap name for the
// CloudEvents status reporter configuration.
func WithCloudEventsStatusReporterConfiguratorConfigMapName(name string) CloudEventsStatusReporterConfiguratorFromConfigMapOption {
	return func(c *cloudEventsStatusReporterConfiguratorFromConfigMap) {
		c.configMapName = name
	}
}

// NewCloudEventsReporterConfiguratorFromConfigMap returns a ConfigMap based CloudEvents
// status reporter configurator.
func NewCloudEventsReporterConfiguratorFromConfigMap(opts ...CloudEventsStatusReporterConfiguratorFromConfigMapOption) CloudEventsStatusReporterConfigurator {
	c := &cloudEventsStatusReporterConfiguratorFromConfigMap{
		configMapName: metrics.ConfigMapName(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// CreateCloudEventsStatusReporter based on the component's ConfigMap.
// The reporter object is returned.
// A Watcher is set to update the configuration dynamically.
func (c *cloudEventsStatusReporterConfiguratorFromConfigMap) CreateCloudEventsStatusReporter(ctx context.Context) *crstatusevent.CRStatusEventClient {
	logger := logging.FromContext(ctx)

	ocm, err := GetConfigMapByPolling(ctx, c.configMapName)
	if err != nil {
		logger.Errorw("observability ConfigMap "+c.configMapName+" could not be retrieved", zap.Error(err))
	}

	var crStatusConfig map[string]string
	if ocm != nil {
		crStatusConfig = ocm.Data
	}
	r := crstatusevent.NewCRStatusEventClient(crStatusConfig)

	logger.Infof("Adding Watcher on ConfigMap %s for CE client status reporter", c.configMapName)
	ConfigWatcherFromContext(ctx).Watch(c.configMapName, crstatusevent.UpdateFromConfigMap(r))

	return r
}

// profilerConfiguratorFromConfigMap dynamically configures
// the profiler using a watcher on a Configmap.
type profilerConfiguratorFromConfigMap struct {
	configMapName string
}

// ProfilerConfiguratorFromConfigMapOption for teawking the profiler configurator.
type ProfilerConfiguratorFromConfigMapOption func(*profilerConfiguratorFromConfigMap)

// WithProfilerConfiguratorConfigMapName sets the ConfigMap name for the profiler configuration.
func WithProfilerConfiguratorConfigMapName(name string) ProfilerConfiguratorFromConfigMapOption {
	return func(c *profilerConfiguratorFromConfigMap) {
		c.configMapName = name
	}
}

// NewProfilerConfiguratorFromConfigMap returns a ConfigMap based profiler configurator.
func NewProfilerConfiguratorFromConfigMap(opts ...ProfilerConfiguratorFromConfigMapOption) ProfilerConfigurator {
	c := &profilerConfiguratorFromConfigMap{
		configMapName: metrics.ConfigMapName(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// CreateProfilingServer creates and return a profiling server that can be enabled or disabled
// depending on the ConfigMap configuration. The server is created either case.
// A Watcher is set to update the configuration dynamically.
func (c *profilerConfiguratorFromConfigMap) CreateProfilingServer(ctx context.Context) *http.Server {
	logger := logging.FromContext(ctx)

	// Configure watcher to update profiler from ConfigMap.
	enabled := false

	ocm, err := GetConfigMapByPolling(ctx, c.configMapName)
	switch {
	case err != nil:
		logger.Errorw("observability ConfigMap "+c.configMapName+" could not be retrieved", zap.Error(err))
	case ocm == nil:
		logger.Warn("profiler configuration not found, falling back to disabling profiler requests")
	default:
		if enabled, err = profiling.ReadProfilingFlag(ocm.Data); err != nil {
			logger.Errorw("wrong profiler configuration")
		}
	}

	// Setup profiler even if it is disabled at the handler. Users
	// might activate it through the ConfigMap.
	profilingHandler := profiling.NewHandler(logger, enabled)
	profilingServer := profiling.NewServer(profilingHandler)

	logger.Infof("Adding Watcher on ConfigMap %s for profiler", c.configMapName)
	ConfigWatcherFromContext(ctx).Watch(c.configMapName, profilingHandler.UpdateFromConfigMap)

	return profilingServer
}
