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

package config

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/eventing/pkg/kncloudevents"
	knconfigmap "knative.dev/pkg/configmap"
)

const (
	// BrokerConfigMap is the name of the configmap for the broker.
	BrokerConfigMap = "config-broker"
	// IngressConfigKey is the key in the "Data" field of the configmap for the broker ingress.
	IngressConfigKey = "ingress-config"
	// FilterConfigKey is the key in the "Data" field of the configmap for the broker filter.
	FilterConfigKey = "filter-config"
)

// DefaultBrokerConfig is the default config for the broker.
var DefaultBrokerConfig = BrokerConfig{
	IngressConfig: IngressConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       2,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			// Defaults for the underlying HTTP Client transport. These would enable better connection reuse.
			// Purposely set them to be equal, as the ingress only connects to its channel.
			// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
			// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 1000,
		},
		TTL: 255,
		MetricsPort: 9092,
	},
	FilterConfig: FilterConfig{
		LivenessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       2,
		},
		ReadinessProbe: corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/readyz",
					Port: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       2,
		},
		ConnectionArgs: kncloudevents.ConnectionArgs{
			// Defaults for the underlying HTTP Client transport. These would enable better connection reuse.
			// Set them on a 10:1 ratio, but this would actually depend on the Triggers' subscribers and the workload itself.
			// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
			// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 100,
		},
		MetricsPort: 9092,
	},
}

// IngressConfig holds the configuration parameters for the broker ingress.
type IngressConfig struct {
	LivenessProbe corev1.Probe
	kncloudevents.ConnectionArgs
	TTL         int32
	MetricsPort int
}

// FilterConfig holds the configuration parameters for the broker filter.
type FilterConfig struct {
	LivenessProbe  corev1.Probe
	ReadinessProbe corev1.Probe
	kncloudevents.ConnectionArgs
	MetricsPort int
}

// BrokerConfig is the in memory representation of the broker configmap.
type BrokerConfig struct {
	IngressConfig IngressConfig
	FilterConfig  FilterConfig
}

// NewConfigFromConfigMap converts a k8s configmap into BrokerConfig.
func NewConfigFromConfigMap(config *corev1.ConfigMap) (BrokerConfig, error) {
	// Initialize with default config so that if a field is missing in the configmap, its default
	// value will be applied.
	c := DefaultBrokerConfig
	err := json.Unmarshal([]byte(config.Data[IngressConfigKey]), &c.IngressConfig)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal([]byte(config.Data[FilterConfigKey]), &c.FilterConfig)
	return c, err
}

// Store loads/unloads untyped configuration from configmap.
type Store struct {
	logger knconfigmap.Logger
	*knconfigmap.UntypedStore
}

// NewStore creates a new broker configuration store.
func NewStore(logger knconfigmap.Logger, onAfterStore ...func(name string, value interface{})) *Store {
	return &Store{
		logger: logger,
		UntypedStore: knconfigmap.NewUntypedStore(
			"broker_config",
			logger,
			knconfigmap.Constructors{
				BrokerConfigMap: NewConfigFromConfigMap,
			},
			onAfterStore...,
		),
	}
}

// GetConfig returns the current config in the store.
func (s *Store) GetConfig() BrokerConfig {
	config := s.UntypedStore.UntypedLoad(BrokerConfigMap)
	if config == nil {
		// This should only happen in tests where we don't watch for the configmap and therefore the
		// store is not populated with data.
		s.logger.Errorf("Failed to load config from store, returning default config: %v.", DefaultBrokerConfig)
		return DefaultBrokerConfig
	}
	return config.(BrokerConfig)
}
