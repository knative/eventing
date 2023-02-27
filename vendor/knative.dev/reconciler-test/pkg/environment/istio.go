/*
Copyright 2023 The Knative Authors

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

package environment

import (
	"context"

	corev1 "k8s.io/api/core/v1"
)

type IstioConfig struct {
	Enabled bool
}

type istioConfigKey struct{}

func withIstioConfig(ctx context.Context, config *IstioConfig) context.Context {
	return context.WithValue(ctx, istioConfigKey{}, config)
}

// GetIstioConfig returns the configured IstioConfig
func GetIstioConfig(ctx context.Context) *IstioConfig {
	config := ctx.Value(istioConfigKey{})
	if config == nil {
		return &IstioConfig{
			Enabled: false,
		}
	}
	return config.(*IstioConfig)
}

func withIstioNamespaceLabel(ns *corev1.Namespace) {
	if ns.Labels == nil {
		ns.Labels = make(map[string]string, 1)
	}
	ns.Labels["istio-injection"] = "enabled"
}

func initIstioFlags() ConfigurationOption {
	return func(configuration Configuration) Configuration {
		fs := configuration.Flags.Get(configuration.Context)

		cfg := &IstioConfig{}

		fs.BoolVar(&cfg.Enabled, "istio.enabled", false, "Enable testing with Istio enabled")

		configuration.Context = withIstioConfig(configuration.Context, cfg)

		return configuration
	}
}
