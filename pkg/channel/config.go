/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package channel

import (
	corev1 "k8s.io/api/core/v1"
	"knative.dev/eventing/pkg/configmap"
	"knative.dev/eventing/pkg/kncloudevents"
	knconfigmap "knative.dev/pkg/configmap"
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
	c := EventDispatcherConfig{}
	requests := []configmap.ReadIntRequest{
		{
			Key:          "MaxIdleConnections",
			DefaultValue: defaultMaxIdleConnections,
			Field:        &c.MaxIdleConns,
		},
		{
			Key:          "MaxIdleConnectionsPerHost",
			DefaultValue: defaultMaxIdleConnectionsPerHost,
			Field:        &c.MaxIdleConnsPerHost,
		},
	}
	err := configmap.ReadInt(requests, config)
	return c, err
}

// EventDispatcherConfigStore loads/unloads untyped configuration from configmap.
type EventDispatcherConfigStore struct {
	logger knconfigmap.Logger
	*knconfigmap.UntypedStore
}

// NewEventDispatcherConfigStore creates a configuration Store
func NewEventDispatcherConfigStore(logger knconfigmap.Logger, onAfterStore ...func(name string, value interface{})) *EventDispatcherConfigStore {
	return &EventDispatcherConfigStore{
		logger: logger,
		UntypedStore: knconfigmap.NewUntypedStore(
			"event_dispatcher",
			logger,
			knconfigmap.Constructors{
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
