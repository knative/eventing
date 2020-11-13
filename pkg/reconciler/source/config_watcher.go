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

package source

import (
	"context"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	tracingconfig "knative.dev/pkg/tracing/config"
)

const (
	EnvLoggingCfg = "K_LOGGING_CONFIG"
	EnvMetricsCfg = "K_METRICS_CONFIG"
	EnvTracingCfg = "K_TRACING_CONFIG"
)

type ConfigAccessor interface {
	ToEnvVars() []corev1.EnvVar
	LoggingConfig() *logging.Config
	MetricsConfig() *metrics.ExporterOptions
	TracingConfig() *tracingconfig.Config
}

var _ ConfigAccessor = (*ConfigWatcher)(nil)

// ConfigWatcher keeps track of logging, metrics and tracing configurations by
// watching corresponding ConfigMaps.
type ConfigWatcher struct {
	logger *zap.SugaredLogger

	component string

	// configurations remain nil if disabled
	loggingCfg *logging.Config
	metricsCfg *metrics.ExporterOptions
	tracingCfg *tracingconfig.Config
}

// configWatcherOption is a function option for ConfigWatchers.
type configWatcherOption func(*ConfigWatcher, configmap.Watcher)

// WatchConfigurations returns a ConfigWatcher initialized with the given
// options. If no option is passed, the ConfigWatcher observes ConfigMaps for
// logging, metrics and tracing.
func WatchConfigurations(loggingCtx context.Context, component string,
	cmw configmap.Watcher, opts ...configWatcherOption) *ConfigWatcher {

	cw := &ConfigWatcher{
		logger:    logging.FromContext(loggingCtx),
		component: component,
	}

	if len(opts) == 0 {
		WithLogging(cw, cmw)
		WithMetrics(cw, cmw)
		WithTracing(cw, cmw)

	} else {
		for _, opt := range opts {
			opt(cw, cmw)
		}
	}

	return cw
}

// WithLogging observes a logging ConfigMap.
func WithLogging(cw *ConfigWatcher, cmw configmap.Watcher) {
	cw.loggingCfg = &logging.Config{}
	watchConfigMap(cmw, logging.ConfigMapName(), cw.updateFromLoggingConfigMap)
}

// WithMetrics observes a metrics ConfigMap.
func WithMetrics(cw *ConfigWatcher, cmw configmap.Watcher) {
	cw.metricsCfg = &metrics.ExporterOptions{}
	watchConfigMap(cmw, metrics.ConfigMapName(), cw.updateFromMetricsConfigMap)
}

// WithTracing observes a tracing ConfigMap.
func WithTracing(cw *ConfigWatcher, cmw configmap.Watcher) {
	cw.tracingCfg = &tracingconfig.Config{}
	watchConfigMap(cmw, tracingconfig.ConfigName, cw.updateFromTracingConfigMap)
}

func watchConfigMap(cmw configmap.Watcher, cmName string, obs configmap.Observer) {
	if dcmw, ok := cmw.(configmap.DefaultingWatcher); ok {
		dcmw.WatchWithDefault(corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: cmName},
			Data:       map[string]string{},
		}, obs)

	} else {
		cmw.Watch(cmName, obs)
	}
}

// LoggingConfig returns the logging configuration from the ConfigWatcher.
func (cw *ConfigWatcher) LoggingConfig() *logging.Config {
	if cw == nil {
		return nil
	}
	return cw.loggingCfg
}

// MetricsConfig returns the metrics configuration from the ConfigWatcher.
func (cw *ConfigWatcher) MetricsConfig() *metrics.ExporterOptions {
	if cw == nil {
		return nil
	}
	return cw.metricsCfg
}

// TracingConfig returns the tracing configuration from the ConfigWatcher.
func (cw *ConfigWatcher) TracingConfig() *tracingconfig.Config {
	if cw == nil {
		return nil
	}
	return cw.tracingCfg
}

func (cw *ConfigWatcher) updateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	delete(cfg.Data, "_example")

	loggingCfg, err := logging.NewConfigFromConfigMap(cfg)
	if err != nil {
		cw.logger.Warnw("failed to create logging config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.loggingCfg = loggingCfg

	cw.logger.Debugw("Updated logging config from ConfigMap", zap.Any("ConfigMap", cfg))
}

func (cw *ConfigWatcher) updateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	delete(cfg.Data, "_example")

	cw.metricsCfg = &metrics.ExporterOptions{
		Domain:    metrics.Domain(),
		Component: cw.component,
		ConfigMap: cfg.Data,
	}

	cw.logger.Debugw("Updated metrics config from ConfigMap", zap.Any("ConfigMap", cfg))
}

func (cw *ConfigWatcher) updateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	delete(cfg.Data, "_example")

	tracingCfg, err := tracingconfig.NewTracingConfigFromMap(cfg.Data)
	if err != nil {
		cw.logger.Warnw("failed to create tracing config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.tracingCfg = tracingCfg

	cw.logger.Debugw("Updated tracing config from ConfigMap", zap.Any("ConfigMap", cfg))
}

// ToEnvVars serializes the contents of the ConfigWatcher to individual
// environment variables.
func (cw *ConfigWatcher) ToEnvVars() []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0, 3)

	envs = maybeAppendEnvVar(envs, cw.loggingConfigEnvVar(), cw.LoggingConfig() != nil)
	envs = maybeAppendEnvVar(envs, cw.metricsConfigEnvVar(), cw.MetricsConfig() != nil)
	envs = maybeAppendEnvVar(envs, cw.tracingConfigEnvVar(), cw.TracingConfig() != nil)

	return envs
}

// maybeAppendEnvVar appends an EnvVar only if the condition boolean is true.
func maybeAppendEnvVar(envs []corev1.EnvVar, env corev1.EnvVar, cond bool) []corev1.EnvVar {
	if !cond {
		return envs
	}

	return append(envs, env)
}

// loggingConfigEnvVar returns an EnvVar containing the serialized logging
// configuration from the ConfigWatcher.
func (cw *ConfigWatcher) loggingConfigEnvVar() corev1.EnvVar {
	cfg, err := logging.ConfigToJSON(cw.LoggingConfig())
	if err != nil {
		cw.logger.Warnw("Error while serializing logging config", zap.Error(err))
	}

	return corev1.EnvVar{
		Name:  EnvLoggingCfg,
		Value: cfg,
	}
}

// metricsConfigEnvVar returns an EnvVar containing the serialized metrics
// configuration from the ConfigWatcher.
func (cw *ConfigWatcher) metricsConfigEnvVar() corev1.EnvVar {
	cfg, err := metrics.OptionsToJSON(cw.MetricsConfig())
	if err != nil {
		cw.logger.Warnw("Error while serializing metrics config", zap.Error(err))
	}

	return corev1.EnvVar{
		Name:  EnvMetricsCfg,
		Value: cfg,
	}
}

// tracingConfigEnvVar returns an EnvVar containing the serialized tracing
// configuration from the ConfigWatcher.
func (cw *ConfigWatcher) tracingConfigEnvVar() corev1.EnvVar {
	cfg, err := tracingconfig.TracingConfigToJSON(cw.TracingConfig())
	if err != nil {
		cw.logger.Warnw("Error while serializing tracing config", zap.Error(err))
	}

	return corev1.EnvVar{
		Name:  EnvTracingCfg,
		Value: cfg,
	}
}

// EmptyVarsGenerator generates empty env vars. Intended to be used in tests.
type EmptyVarsGenerator struct {
	ConfigAccessor
}

var _ ConfigAccessor = (*EmptyVarsGenerator)(nil)

func (g *EmptyVarsGenerator) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: EnvLoggingCfg},
		{Name: EnvMetricsCfg},
		{Name: EnvTracingCfg},
	}
}
