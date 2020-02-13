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
package adapter

type EnvConfigConstructor func() EnvConfigAccessor

// EnvConfig is the minimal set of configuration parameters
// source adapters should support.
type EnvConfig struct {
	// SinkURI is the URI messages will be forwarded to.
	// DEPRECATED: use K_SINK
	SinkURI string `envconfig:"SINK_URI"`

	// Sink is the URI messages will be sent.
	Sink string `envconfig:"K_SINK"`

	// Environment variable containing the namespace of the adapter.
	Namespace string `envconfig:"NAMESPACE" required:"true"`

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
	// Get the URI where messages will be forwarded to.
	GetSinkURI() string

	// Get the namespace of the adapter.
	GetNamespace() string

	// Get the json string of metrics.ExporterOptions.
	GetMetricsConfigJson() string

	// Get the json string of logging.Config.
	GetLoggingConfigJson() string
}

func (e *EnvConfig) GetMetricsConfigJson() string {
	return e.MetricsConfigJson
}

func (e *EnvConfig) GetLoggingConfigJson() string {
	return e.LoggingConfigJson
}

func (e *EnvConfig) GetSinkURI() string {
	if e.Sink != "" {
		return e.Sink
	}
	return e.SinkURI
}

func (e *EnvConfig) GetNamespace() string {
	return e.Namespace
}
