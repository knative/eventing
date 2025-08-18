/*
Copyright 2025 The Knative Authors

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

package observability

import (
	"context"
	"fmt"

	configmap "knative.dev/pkg/configmap/parser"
	pkgo11y "knative.dev/pkg/observability"
	"knative.dev/pkg/observability/metrics"
)

const (
	// EnableSinkEventErrorReportingKey is the CM key to enable emitting a k8s event when delivery to a sink fails
	EnableSinkEventErrorReportingKey = "sink-event-error-reporting.enable"

	// DefaultEnableSinkEventErrorReporting is used to set the default sink event error reporting value
	DefaultEnableSinkEventErrorReporting = false

	// DefaultMetricsPort is the default port used for prometheus metrics if the prometheus protocol is used
	DefaultMetricsPort = 9092
)

type (
	BaseConfig    = pkgo11y.Config
	MetricsConfig = pkgo11y.MetricsConfig
	RuntimeConfig = pkgo11y.RuntimeConfig
	TracingConfig = pkgo11y.TracingConfig
)

// +k8s:deepcopy-gen=true
type Config struct {
	BaseConfig

	// EnableSinkEventErrorReporting specifies whether we should emit a k8s
	// event when delivery to a sink fails
	EnableSinkEventErrorReporting bool `json:"enableSinkEventErrorReporting"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseConfig:                    *pkgo11y.DefaultConfig(),
		EnableSinkEventErrorReporting: DefaultEnableSinkEventErrorReporting,
	}
}

func NewFromMap(m map[string]string) (*Config, error) {
	c := DefaultConfig()

	if cfg, err := pkgo11y.NewFromMap(m); err != nil {
		fmt.Printf("failed to parse config from map: %s\n", err.Error())
		return nil, err
	} else {
		c.BaseConfig = *cfg
	}

	// Force the port to the default queue user metrics port if it's not overridden
	if c.BaseConfig.Metrics.Protocol == metrics.ProtocolPrometheus && c.BaseConfig.Metrics.Endpoint == "" {
		c.BaseConfig.Metrics.Endpoint = fmt.Sprintf(":%d", DefaultMetricsPort)
	}

	err := configmap.Parse(m, configmap.As(EnableSinkEventErrorReportingKey, &c.EnableSinkEventErrorReporting))
	if err != nil {
		fmt.Printf("failed to parse enable-sink-error-reporting: %s\n", err.Error())
		return c, err
	}

	return c, nil
}

type cfgKey struct{}

func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, cfg)
}

func GetConfig(ctx context.Context) *Config {
	// we can ignore the "ok" value here, because the config will just be nil in that case
	cfg, _ := ctx.Value(cfgKey{}).(*Config)
	return cfg
}

// merge the config with the default config, setting defaults wherever config is not set
func MergeWithDefaults(cfg *Config) *Config {
	d := DefaultConfig()

	if cfg == nil {
		return d
	}

	var emptyMetrics MetricsConfig
	if cfg.Metrics == emptyMetrics {
		cfg.Metrics = d.Metrics
	}

	// Force the port to the default queue user metrics port if it's not overridden
	if cfg.BaseConfig.Metrics.Protocol == metrics.ProtocolPrometheus && cfg.BaseConfig.Metrics.Endpoint == "" {
		cfg.BaseConfig.Metrics.Endpoint = fmt.Sprintf(":%d", DefaultMetricsPort)
	}

	var emptyRuntime RuntimeConfig
	if cfg.Runtime == emptyRuntime {
		cfg.Runtime = d.Runtime
	}

	var emptyTracing TracingConfig
	if cfg.Tracing == emptyTracing {
		cfg.Tracing = d.Tracing
	}

	return cfg
}
