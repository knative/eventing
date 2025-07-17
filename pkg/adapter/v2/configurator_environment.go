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

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"knative.dev/pkg/logging"
	k8sruntime "knative.dev/pkg/observability/runtime/k8s"

	"knative.dev/eventing/pkg/adapter/v2/util/crstatusevent"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/observability/otel"
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
	cfg, err := c.env.GetObservabilityConfig()
	if err != nil {
		logger.Warn("CloudEvents client reporter not configured", zap.Error(err))
		return nil
	}

	return crstatusevent.NewCRStatusEventClient(cfg)
}

type observabilityConfiguratorFromEnvironment struct {
	env EnvConfigAccessor
}

func NewObservabilityConfiguratorFromEnvironment(env EnvConfigAccessor) ObservabilityConfigurator {
	return &observabilityConfiguratorFromEnvironment{
		env: env,
	}
}

func (o *observabilityConfiguratorFromEnvironment) SetupObservabilityOrDie(ctx context.Context, component string, logger *zap.SugaredLogger, pprof *k8sruntime.ProfilingServer) (metric.MeterProvider, trace.TracerProvider) {
	cfg, err := o.env.GetObservabilityConfig()
	if err != nil {
		logger.Warnw("failed to parse observability config from env, falling back to defaults", zap.Error(err))
	}

	cfg = observability.MergeWithDefaults(cfg)

	ctx = observability.WithConfig(ctx, cfg)
	return otel.SetupObservabilityOrDie(ctx, component, logger, pprof)
}
