/*
Copyright 2019 The Knative Authors

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

package tracing

import (
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	eventingconfigmap "knative.dev/eventing/pkg/configmap"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"
)

// TODO Move this to knative/pkg.

var (
	// OnePercentSampling is a configuration that samples 1% of the requests.
	// TODO(#1712): Remove this and pull "static" configuration from the
	// environment instead.
	OnePercentSampling = &tracingconfig.Config{
		Backend:        tracingconfig.Zipkin,
		Debug:          false,
		SampleRate:     0.01,
		ZipkinEndpoint: "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
	}

	// AlwaysSample is a configuration that samples 100% of the requests and sends them to Zipkin.
	// It is expected to be used only for testing purposes (e.g. in e2e tests).
	// TODO(#1712): Remove this and pull "static" configuration from the environment instead.
	AlwaysSample = &tracingconfig.Config{
		Backend:        tracingconfig.Zipkin,
		Debug:          true,
		SampleRate:     1.0,
		ZipkinEndpoint: "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
	}
)

// setupPublishing sets up trace publishing for the process. Note that other pieces
// still need to generate the traces, this just ensures that if generated, they are collected
// appropriately. This is normally done by using tracing.HTTPSpanMiddleware as a middleware HTTP
// handler.
func setupPublishing(serviceName string, logger *zap.SugaredLogger) *tracing.OpenCensusTracer {
	return tracing.NewOpenCensusTracer(tracing.WithExporter(serviceName, logger))
}

// SetupStaticPublishing sets up trace publishing for the process. Note that other
// pieces still need to generate the traces, this just ensures that if generated, they are collected
// appropriately. This is normally done by using tracing.HTTPSpanMiddleware as a middleware HTTP
// handler. The configuration will not be dynamically updated.
func SetupStaticPublishing(logger *zap.SugaredLogger, serviceName string, cfg *tracingconfig.Config) error {
	oct := setupPublishing(serviceName, logger)
	err := oct.ApplyConfig(cfg)
	if err != nil {
		return fmt.Errorf("unable to set OpenCensusTracing config: %v", err)
	}
	return nil
}

// SetupDynamicPublishing sets up trace publishing for the process, by watching a
// ConfigMap for the configuration. Note that other pieces still need to generate the traces, this
// just ensures that if generated, they are collected appropriately. This is normally done by using
// tracing.HTTPSpanMiddleware as a middleware HTTP handler. The configuration will be dynamically
// updated when the ConfigMap is updated.
func SetupDynamicPublishing(logger *zap.SugaredLogger, configMapWatcher *configmap.InformedWatcher, serviceName, tracingConfigName string) error {
	oct := setupPublishing(serviceName, logger)

	tracerUpdater := func(name string, value interface{}) {
		if name == tracingConfigName {
			cfg := value.(*tracingconfig.Config)
			logger.Debugw("Updating tracing config", zap.Any("cfg", cfg))
			err := oct.ApplyConfig(cfg)
			if err != nil {
				logger.Errorw("Unable to apply open census tracer config", zap.Error(err))
				return
			}
		}
	}

	// Set up our config store.
	configStore := eventingconfigmap.NewDefaultUntypedStore(
		"tracing-config",
		logger,
		[]eventingconfigmap.DefaultConstructor{
			{
				Default:     enableZeroSamplingCM(configMapWatcher.Namespace, tracingConfigName),
				Constructor: tracingconfig.NewTracingConfigFromConfigMap,
			},
		},
		tracerUpdater)
	configStore.WatchConfigs(configMapWatcher)
	return nil
}

func enableZeroSamplingCM(ns string, tracingConfigName string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tracingConfigName,
			Namespace: ns,
		},
		Data: map[string]string{
			"backend":         "zipkin",
			"debug":           "False",
			"sample-rate":     "0",
			"zipkin-endpoint": "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
		},
	}
}
