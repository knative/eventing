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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	// PingDefaultsConfigName is the name of config map for the default
	// configs that pings should use.
	PingDefaultsConfigName = "config-ping-webhook"

	// PingDefaulterKey is the key in the ConfigMap to get the name of the default
	// Ping CRD.
	PingDefaulterKey = "ping-config"

	PingDataMaxSize = -1
)

// NewPingDefaultsConfigFromMap creates a Defaults from the supplied Map
func NewPingDefaultsConfigFromMap(data map[string]string) (*PingDefaults, error) {
	nc := &PingDefaults{DataMaxSize: PingDataMaxSize}

	// Parse out the Broker Configuration Cluster default section
	value, present := data[PingDefaulterKey]
	if !present || value == "" {
		return nil, fmt.Errorf("ConfigMap is missing (or empty) key: %q : %v", PingDefaulterKey, data)
	}
	if err := parseEntry(value, nc); err != nil {
		return nil, fmt.Errorf("Failed to parse the entry: %s", err)
	}
	return nc, nil
}

func parseEntry(entry string, out interface{}) error {
	j, err := yaml.YAMLToJSON([]byte(entry))
	if err != nil {
		return fmt.Errorf("ConfigMap's value could not be converted to JSON: %s : %v", err, entry)
	}
	return json.Unmarshal(j, &out)
}

// NewPingDefaultsConfigFromConfigMap creates a PingDefaults from the supplied configMap
func NewPingDefaultsConfigFromConfigMap(config *corev1.ConfigMap) (*PingDefaults, error) {
	return NewPingDefaultsConfigFromMap(config.Data)
}

// PingDefaults includes the default values to be populated by the webhook.
type PingDefaults struct {
	DataMaxSize int `json:"dataMaxSize"`
}

func (d *PingDefaults) GetPingConfig() *PingDefaults {
	if d.DataMaxSize < 0 {
		d.DataMaxSize = PingDataMaxSize
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
