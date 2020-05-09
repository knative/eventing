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

	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/configmap"
	pkglogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	tracingconfig "knative.dev/pkg/tracing/config"
)

const (
	EnvLoggingCfg = "K_LOGGING_CONFIG"
	EnvMetricsCfg = "K_METRICS_CONFIG"
	EnvTracingCfg = "K_TRACING_CONFIG"
)

type EnvVarsGenerator interface {
	ToEnvVars() []corev1.EnvVar
}

var _ EnvVarsGenerator = (*ConfigWatcher)(nil)

// ConfigWatcher keeps track of logging, metrics and tracing configurations by
// watching corresponding ConfigMaps.
type ConfigWatcher struct {
	logger *zap.Logger

	component string

	loggingCfg *pkglogging.Config
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

// StartWatchingSourceConfigurations runs WatchConfigurations with all configurations.
// For backwards compatibility only.
func StartWatchingSourceConfigurations(loggingCtx context.Context, component string, cmw configmap.Watcher) *ConfigWatcher {
	return WatchConfigurations(loggingCtx, component, cmw,
		WithLogging,
		WithMetrics,
		WithTracing,
	)
}

// WithLogging observes a logging ConfigMap.
func WithLogging(cw *ConfigWatcher, cmw configmap.Watcher) {
	watchConfigMap(cmw, pkglogging.ConfigMapName(), cw.updateFromLoggingConfigMap)
}

// WithMetrics observes a metrics ConfigMap.
func WithMetrics(cw *ConfigWatcher, cmw configmap.Watcher) {
	watchConfigMap(cmw, metrics.ConfigMapName(), cw.updateFromMetricsConfigMap)
}

// WithTracing observes a tracing ConfigMap.
func WithTracing(cw *ConfigWatcher, cmw configmap.Watcher) {
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
func (cw *ConfigWatcher) LoggingConfig() *pkglogging.Config {
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

	loggingCfg, err := pkglogging.NewConfigFromConfigMap(cfg)
	if err != nil {
		cw.logger.Warn("failed to create logging config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.loggingCfg = loggingCfg

	cw.logger.Debug("Updated logging config from ConfigMap", zap.Any("ConfigMap", cfg))
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

	cw.logger.Debug("Updated metrics config from ConfigMap", zap.Any("ConfigMap", cfg))
}

func (cw *ConfigWatcher) updateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg == nil {
		return
	}

	delete(cfg.Data, "_example")

	tracingCfg, err := tracingconfig.NewTracingConfigFromMap(cfg.Data)
	if err != nil {
		cw.logger.Warn("failed to create tracing config from ConfigMap", zap.String("cfg.Name", cfg.Name))
		return
	}

	cw.tracingCfg = tracingCfg

	cw.logger.Debug("Updated tracing config from ConfigMap", zap.Any("ConfigMap", cfg))
}

// ToEnvVars serializes the contents of the ConfigWatcher to individual
// environment variables.
func (r *ConfigWatcher) ToEnvVars() []corev1.EnvVar {
	envs := make([]corev1.EnvVar, 0, 3)

	loggingCfg, err := pkglogging.LoggingConfigToJson(r.LoggingConfig())
	if err != nil {
		r.logger.Warn("Error while serializing logging config", zap.Error(err))
	}

	metricsCfg, err := metrics.MetricsOptionsToJson(r.MetricsConfig())
	if err != nil {
		r.logger.Warn("Error while serializing metrics config", zap.Error(err))
	}

	tracingCfg, err := tracingconfig.TracingConfigToJson(r.TracingConfig())
	if err != nil {
		r.logger.Warn("Error while serializing tracing config", zap.Error(err))
	}

	envs = maybeAppendEnvVar(envs, EnvLoggingCfg, loggingCfg, r.LoggingConfig() != nil)
	envs = maybeAppendEnvVar(envs, EnvMetricsCfg, metricsCfg, r.MetricsConfig() != nil)
	envs = maybeAppendEnvVar(envs, EnvTracingCfg, tracingCfg, r.TracingConfig() != nil)

	return envs
}

// maybeAppendEnvVar appends an EnvVar with the given name and value only if
// the condition boolean is true.
func maybeAppendEnvVar(envs []corev1.EnvVar, name, val string, cond bool) []corev1.EnvVar {
	if !cond {
		return envs
	}

	return append(envs, corev1.EnvVar{
		Name:  name,
		Value: val,
	})
}

// EmptyVarsGenerator generates empty env vars. Intended to be used in tests.
type EmptyVarsGenerator struct{}

var _ EnvVarsGenerator = (*EmptyVarsGenerator)(nil)

// ToEnvVars implements EnvVarsGenerator.
func (*EmptyVarsGenerator) ToEnvVars() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: EnvLoggingCfg},
		{Name: EnvMetricsCfg},
		{Name: EnvTracingCfg},
	}
}
