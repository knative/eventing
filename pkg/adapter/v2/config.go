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
package adapter

import (
	"encoding/json"

	"go.uber.org/zap"

	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
)

type EnvConfigConstructor func() EnvConfigAccessor

// EnvConfig is the minimal set of configuration parameters
// source adapters should support.
type EnvConfig struct {
	// Component is the kind of this adapter.
	Component string `envconfig:"K_COMPONENT"`

	// Environment variable containing the namespace of the adapter.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

	// Environment variable containing the name of the adapter.
	Name string `envconfig:"NAME" default:"adapter"`

	// Environment variable containing the resource group of the adapter for metrics.
	ResourceGroup string `envconfig:"K_RESOURCE_GROUP" default:"adapter.sources.knative.dev"`

	// Sink is the URI messages will be sent.
	Sink string `envconfig:"K_SINK"`

	// CEOverrides are the CloudEvents overrides to be applied to the outbound event.
	CEOverrides string `envconfig:"K_CE_OVERRIDES"`

	// MetricsConfigJson is a json string of metrics.ExporterOptions.
	// This is used to configure the metrics exporter options,
	// the config is stored in a config map inside the controllers
	// namespace and copied here.
	MetricsConfigJson string `envconfig:"K_METRICS_CONFIG" required:"true"`

	// LoggingConfigJson is a json string of logging.Config.
	// This is used to configure the logging config, the config is stored in
	// a config map inside the controllers namespace and copied here.
	LoggingConfigJson string `envconfig:"K_LOGGING_CONFIG" required:"true"`
}

// EnvConfigAccessor defines accessors for the minimal
// set of source adapter configuration parameters.
type EnvConfigAccessor interface {
	// Set the component name.
	SetComponent(string)

	// Get the URI where messages will be forwarded to.
	GetSink() string

	// Get the namespace of the adapter.
	GetNamespace() string

	// Get the name of the adapter.
	GetName() string

	// Get the parsed metrics.ExporterOptions.
	GetMetricsConfig() (*metrics.ExporterOptions, error)

	// Get the parsed logger.
	GetLogger() *zap.SugaredLogger

	GetCloudEventOverrides() (*duckv1.CloudEventOverrides, error)
}

var _ EnvConfigAccessor = (*EnvConfig)(nil)

func (e *EnvConfig) SetComponent(component string) {
	e.Component = component
}

func (e *EnvConfig) GetMetricsConfig() (*metrics.ExporterOptions, error) {
	// Convert json metrics.ExporterOptions to metrics.ExporterOptions.
	metricsConfig, err := metrics.JsonToMetricsOptions(e.MetricsConfigJson)
	if err != nil {
		return nil, err
	}
	return metricsConfig, err
}

func (e *EnvConfig) GetLogger() *zap.SugaredLogger {
	loggingConfig, err := logging.JsonToLoggingConfig(e.LoggingConfigJson)
	if err != nil {
		// Use default logging config.
		if loggingConfig, err = logging.NewConfigFromMap(map[string]string{}); err != nil {
			// If this fails, there is no recovering.
			panic(err)
		}
	}

	logger, _ := logging.NewLoggerFromConfig(loggingConfig, e.Component)

	return logger
}

func (e *EnvConfig) GetSink() string {
	return e.Sink
}

func (e *EnvConfig) GetNamespace() string {
	return e.Namespace
}

func (e *EnvConfig) GetName() string {
	return e.Name
}

func (e *EnvConfig) GetCloudEventOverrides() (*duckv1.CloudEventOverrides, error) {
	var ceOverrides duckv1.CloudEventOverrides
	if len(e.CEOverrides) > 0 {
		err := json.Unmarshal([]byte(e.CEOverrides), &ceOverrides)
		if err != nil {
			return nil, err
		}
	}
	return &ceOverrides, nil
}
