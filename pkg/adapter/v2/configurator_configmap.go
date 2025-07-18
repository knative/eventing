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
	"os"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/observability"
	o11yconfigmap "knative.dev/eventing/pkg/observability/configmap"
	"knative.dev/eventing/pkg/observability/otel"
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

type observabilityConfiguratorFromConfigMap struct {
	configMapName string
}

type ObservabilityConfiguratorFromConfigMapOption func(*observabilityConfiguratorFromConfigMap)

func WithObservabilityConfiguratorConfigMapName(name string) ObservabilityConfiguratorFromConfigMapOption {
	return func(ocfcm *observabilityConfiguratorFromConfigMap) {
		ocfcm.configMapName = name
	}
}

func NewObservabilityConfiguratorFromConfigMap(opts ...ObservabilityConfiguratorFromConfigMapOption) ObservabilityConfigurator {
	o := &observabilityConfiguratorFromConfigMap{
		configMapName: o11yconfigmap.Name(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *observabilityConfiguratorFromConfigMap) SetupObservabilityOrDie(ctx context.Context, component string, logger *zap.SugaredLogger, pprof *k8sruntime.ProfilingServer) (metric.MeterProvider, trace.TracerProvider) {
	ocm, err := GetConfigMapByPolling(ctx, o.configMapName)
	if err != nil {
		logger.Panicw("observability ConfigMap "+o.configMapName+" could not be retrieved", zap.Error(err))
	}

	cfg, err := o11yconfigmap.Parse(ocm)
	if err != nil {
		logger.Panicw("failed to parse config from configmap", zap.Error(err))
	}

	cfg = observability.MergeWithDefaults(cfg)

	ctx = observability.WithConfig(ctx, cfg)

	return otel.SetupObservabilityOrDie(ctx, component, logger, pprof)
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
		configMapName: o11yconfigmap.Name(),
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

	cfg, err := o11yconfigmap.Parse(ocm)
	if err != nil {
		logger.Errorw("observability config could not be parsed from ConfigMap, falling back to default (noop) config", zap.Error(err))
		cfg = observability.DefaultConfig()
	}
	r := crstatusevent.NewCRStatusEventClient(cfg)

	logger.Infof("Adding Watcher on ConfigMap %s for CE client status reporter", c.configMapName)
	ConfigWatcherFromContext(ctx).Watch(c.configMapName, crstatusevent.UpdateFromConfigMap(r))

	return r
}
