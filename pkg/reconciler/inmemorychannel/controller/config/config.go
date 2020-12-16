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
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"

	"knative.dev/eventing/pkg/kncloudevents"
)

const (
	// Defaults for the underlying HTTP Client transport. These would enable better connection reuse.
	// Set them on a 10:1 ratio, but this would actually depend on the Subscriptions' subscribers and the workload itself.
	// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
	// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 100
)

// EventDispatcherConfigMap is the name of the configmap for event dispatcher.
const EventDispatcherConfigMap = "config-imc-event-dispatcher"

var defaultEventDispatcherConfig = EventDispatcherConfig{
	ConnectionArgs: kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	},
}

// EventDispatcherConfig holds the configuration parameters for the event dispatcher.
type EventDispatcherConfig struct {
	kncloudevents.ConnectionArgs
}

// NewEventDisPatcherConfigFromConfigMap converts a k8s configmap into EventDispatcherConfig.
func NewEventDisPatcherConfigFromConfigMap(config *corev1.ConfigMap) (EventDispatcherConfig, error) {
	c := EventDispatcherConfig{
		ConnectionArgs: kncloudevents.ConnectionArgs{
			MaxIdleConns:        defaultMaxIdleConnections,
			MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
		},
	}
	err := configmap.Parse(
		config.Data,
		configmap.AsInt("MaxIdleConnections", &c.MaxIdleConns),
		configmap.AsInt("MaxIdleConnectionsPerHost", &c.MaxIdleConnsPerHost))
	return c, err
}

// EventDispatcherConfigStore loads/unloads untyped configuration from configmap.
type EventDispatcherConfigStore struct {
	logger configmap.Logger
	*configmap.UntypedStore
}

// NewEventDispatcherConfigStore creates a configuration Store
func NewEventDispatcherConfigStore(logger configmap.Logger, onAfterStore ...func(name string, value interface{})) *EventDispatcherConfigStore {
	return &EventDispatcherConfigStore{
		logger: logger,
		UntypedStore: configmap.NewUntypedStore(
			"event_dispatcher",
			logger,
			configmap.Constructors{
				EventDispatcherConfigMap: NewEventDisPatcherConfigFromConfigMap,
			},
			onAfterStore...,
		),
	}
}

// GetConfig returns the current config in the store.
func (s *EventDispatcherConfigStore) GetConfig() EventDispatcherConfig {
	config := s.UntypedStore.UntypedLoad(EventDispatcherConfigMap)
	if config == nil {
		// This should only happen in tests where we don't watch for the configmap and therefore the
		// store is not populated with data.
		s.logger.Errorf("Failed to load config from store, return default config: %v.", defaultEventDispatcherConfig)
		return defaultEventDispatcherConfig
	}
	return config.(EventDispatcherConfig)
}
