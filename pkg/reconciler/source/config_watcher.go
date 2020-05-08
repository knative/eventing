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
	pkgLogging "knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/logging"
)

type ConfigWatcher struct {
	logger *zap.Logger

	component string

	loggingConfig *pkgLogging.Config
	metricsConfig *metrics.ExporterOptions
	tracingCfg    *tracingconfig.Config
}

func StartWatchingSourceConfigurations(loggingContext context.Context, component string, cmw configmap.Watcher) *ConfigWatcher {
	cw := ConfigWatcher{
		logger:    logging.FromContext(loggingContext),
		component: component,
	}

	if dcmw, ok := cmw.(configmap.DefaultingWatcher); ok {
		dcmw.WatchWithDefault(corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: pkgLogging.ConfigMapName()},
			Data:       map[string]string{},
		}, cw.UpdateFromLoggingConfigMap)
		dcmw.WatchWithDefault(corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: metrics.ConfigMapName()},
			Data:       map[string]string{},
		}, cw.UpdateFromMetricsConfigMap)
		dcmw.WatchWithDefault(corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: tracingconfig.ConfigName},
			Data:       map[string]string{},
		}, cw.UpdateFromTracingConfigMap)
	} else {
		cmw.Watch(pkgLogging.ConfigMapName(), cw.UpdateFromLoggingConfigMap)
		cmw.Watch(metrics.ConfigMapName(), cw.UpdateFromMetricsConfigMap)
		cmw.Watch(tracingconfig.ConfigName, cw.UpdateFromTracingConfigMap)
	}

	return &cw
}

func (r *ConfigWatcher) UpdateFromLoggingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")

		logcfg, err := pkgLogging.NewConfigFromConfigMap(cfg)
		if err != nil {
			r.logger.Warn("failed to create logging config from configmap", zap.String("cfg.Name", cfg.Name))
			return
		}
		r.loggingConfig = logcfg
		r.logger.Debug("Update from logging ConfigMap", zap.Any("ConfigMap", cfg))
	}
}

func (r *ConfigWatcher) UpdateFromMetricsConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")

		r.metricsConfig = &metrics.ExporterOptions{
			Domain:    metrics.Domain(),
			Component: r.component,
			ConfigMap: cfg.Data,
		}
		r.logger.Debug("Update from metrics ConfigMap", zap.Any("ConfigMap", cfg))
	}
}

func (r *ConfigWatcher) UpdateFromTracingConfigMap(cfg *corev1.ConfigMap) {
	if cfg != nil {
		delete(cfg.Data, "_example")

		tracingCfg, err := tracingconfig.NewTracingConfigFromMap(cfg.Data)
		if err != nil {
			r.logger.Warn("failed to create tracing config from configmap", zap.String("cfg.Name", cfg.Name))
			return
		}

		r.tracingCfg = tracingCfg
		r.logger.Debug("Update from tracing ConfigMap", zap.Any("ConfigMap", cfg))
	}
}

func (r *ConfigWatcher) LoggingConfig() *pkgLogging.Config {
	if r == nil {
		return nil
	}
	return r.loggingConfig
}

func (r *ConfigWatcher) MetricsConfig() *metrics.ExporterOptions {
	if r == nil {
		return nil
	}
	return r.metricsConfig
}

func (r *ConfigWatcher) TracingConfig() *tracingconfig.Config {
	if r == nil {
		return nil
	}
	return r.tracingCfg
}
