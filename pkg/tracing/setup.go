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

	"github.com/knative/pkg/configmap"
	"github.com/knative/pkg/tracing"
	tracingconfig "github.com/knative/pkg/tracing/config"
	zipkin "github.com/openzipkin/zipkin-go"
	"go.uber.org/zap"
)

// TODO Move this to knative/pkg.

var (
	// DebugCfg is a configuration to use to record all traces.
	DebugCfg = &tracingconfig.Config{
		Enable:         true,
		Debug:          true,
		SampleRate:     1,
		ZipkinEndpoint: "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
	}
	// EnabledZeroSampling is a configuration that enables tracing, but has a sampling rate of zero.
	// The intention is that this allows this to record traces that other components started, but
	// will not start traces itself.
	EnabledZeroSampling = &tracingconfig.Config{
		Enable:         true,
		Debug:          false,
		SampleRate:     0,
		ZipkinEndpoint: "http://zipkin.istio-system.svc.cluster.local:9411/api/v2/spans",
	}
)

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
func SetupDynamicZipkinPublishing(logger *zap.SugaredLogger, configMapWatcher configmap.Watcher, serviceName string) error {
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
	configStore := configmap.NewUntypedStore(
		"tracing-config",
		logger,
		configmap.Constructors{
			tracingconfig.ConfigName: tracingconfig.NewTracingConfigFromConfigMap,
		},
		tracerUpdater)
	configStore.WatchConfigs(configMapWatcher)
	return nil
}
