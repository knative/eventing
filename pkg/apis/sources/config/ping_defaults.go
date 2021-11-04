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

	cm "knative.dev/pkg/configmap"
)

const (
	// PingDefaultsConfigName is the name of config map for the default
	// configs that pings should use.
	PingDefaultsConfigName = "config-ping-defaults"

	DataMaxSizeKey = "data-max-size"

	// Legacy configuration item should be removed when a migration
	// update script is released.
	LegacyDataMaxSizeKey = "dataMaxSize"

	DefaultDataMaxSize = -1
)

// NewPingDefaultsConfigFromMap creates a Defaults from the supplied Map
func NewPingDefaultsConfigFromMap(data map[string]string) (*PingDefaults, error) {
	nc := &PingDefaults{DataMaxSize: DefaultDataMaxSize}

	if err := cm.Parse(data,
		// Legacy for backwards compatibility
		cm.AsInt64(LegacyDataMaxSizeKey, &nc.DataMaxSize),

		cm.AsInt64(DataMaxSizeKey, &nc.DataMaxSize),
	); err != nil {
		return nil, err
	}

	return nc, nil
}

// NewPingDefaultsConfigFromConfigMap creates a PingDefaults from the supplied configMap
func NewPingDefaultsConfigFromConfigMap(config *corev1.ConfigMap) (*PingDefaults, error) {
	return NewPingDefaultsConfigFromMap(config.Data)
}

// PingDefaults includes the default values to be populated by the webhook.
type PingDefaults struct {
	DataMaxSize int64 `json:"data-max-size"`
}

func (d *PingDefaults) GetPingConfig() *PingDefaults {
	if d.DataMaxSize < 0 {
		d.DataMaxSize = DefaultDataMaxSize
	}
	return d

}

func (d *PingDefaults) DeepCopy() *PingDefaults {
	if d == nil {
		return nil
	}
	out := new(PingDefaults)
	*out = *d
	return out
}
