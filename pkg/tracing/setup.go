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

	eventingconfigmap "github.com/knative/eventing/pkg/configmap"
	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/tracing"
	tracingconfig "github.com/knative/pkg/tracing/config"
	zipkin "github.com/openzipkin/zipkin-go"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO Move this to knative/pkg.

// setupZipkinPublishing sets up Zipkin trace publishing for the process. Note that other pieces
// still need to generate the traces, this just ensures that if generated, they are collected
// appropriately. This is normally done by using tracing.HTTPSpanMiddleware as a middleware HTTP
// handler.
func setupZipkinPublishing(serviceName string) (*tracing.OpenCensusTracer, error) {
	// TODO Should we fill in the hostPort?
	zipkinEndpoint, err := zipkin.NewEndpoint(serviceName, "")
	if err != nil {
		return nil, fmt.Errorf("unable to create tracing endpoint: %v", err)
	}
	oct := tracing.NewOpenCensusTracer(tracing.WithZipkinExporter(tracing.CreateZipkinReporter, zipkinEndpoint))
	return oct, nil
}

// SetupStaticZipkinPublishing sets up Zipkin trace publishing for the process. Note that other
// pieces still need to generate the traces, this just ensures that if generated, they are collected
// appropriately. This is normally done by using tracing.HTTPSpanMiddleware as a middleware HTTP
// handler. The configuration will not be dynamically updated.
func SetupStaticZipkinPublishing(serviceName string, cfg *tracingconfig.Config) error {
	oct, err := setupZipkinPublishing(serviceName)
	if err != nil {
		return err
	}
	err = oct.ApplyConfig(cfg)
	if err != nil {
		return fmt.Errorf("unable to set OpenCensusTracing config: %v", err)
	}
	return nil
}

// SetupDynamicZipkinPublishing sets up Zipkin trace publishing for the process, by watching a
// ConfigMap for the configuration. Note that other pieces still need to generate the traces, this
// just ensures that if generated, they are collected appropriately. This is normally done by using
// tracing.HTTPSpanMiddleware as a middleware HTTP handler. The configuration will be dynamically
// updated when the ConfigMap is updated.
func SetupDynamicZipkinPublishing(logger *zap.SugaredLogger, configMapWatcher *configmap.InformedWatcher, serviceName string) error {
	oct, err := setupZipkinPublishing(serviceName)
	if err != nil {
		return err
	}

	tracerUpdater := func(name string, value interface{}) {
		if name == tracingconfig.ConfigName {
			cfg := value.(*tracingconfig.Config)
			logger.Debugw("Updating tracing config", zap.Any("cfg", cfg))
			err = oct.ApplyConfig(cfg)
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
				Default:     enableZeroSamplingCM(configMapWatcher.Namespace),
				Constructor: tracingconfig.NewTracingConfigFromConfigMap,
			},
		},
		tracerUpdater)
	configStore.WatchConfigs(configMapWatcher)
	return nil
}

func enableZeroSamplingCM(ns string) corev1.ConfigMap {
	return corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tracingconfig.ConfigName,
			Namespace: ns,
		},
		Data: map[string]string{
			"enable":          "True",
			"debug":           "False",
			"sample-rate":     "0",
			"zipkin-endpoint": "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
		},
	}
}
