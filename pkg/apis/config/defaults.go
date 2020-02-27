/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	// DefaultConfigName is the name of config map for the default
	// configs that brokers should use
	DefaultsConfigName = "config-br-defaults"

	// BrokerDefaultsKey is the name of the key that's used for finding
	// defaults for broker configs.
	BrokerDefaultsKey = "default-br-config"
)

// NewDefaultsConfigFromMap creates a Defaults from the supplied Map
func NewDefaultsConfigFromMap(data map[string]string) (*Defaults, error) {
	nc := &Defaults{}

	// Parse out the Broker Configuration Cluster default section
	value, present := data[BrokerDefaultsKey]
	if !present || value == "" {
		return nil, fmt.Errorf("ConfigMap is missing (or empty) key: %q : %v", BrokerDefaultsKey, data)
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
	return json.Unmarshal([]byte(j), &out)
}

// NewDefaultsConfigFromConfigMap creates a Defaults from the supplied configMap
func NewDefaultsConfigFromConfigMap(config *corev1.ConfigMap) (*Defaults, error) {
	return NewDefaultsConfigFromMap(config.Data)
}

// Defaults includes the default values to be populated by the webhook.
type Defaults struct {
	// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
	// Namespace is the key, the value is the KReference to the config.
	NamespaceDefaultsConfig map[string]*duckv1.KReference `json:"namespaceDefaults,omitempty"`

	// ClusterDefaultBrokerConfig is the default broker config for all the namespaces that
	// are not in NamespaceDefaultBrokerConfigs.
	ClusterDefault *duckv1.KReference `json:"clusterDefault,omitempty"`
}

// GetBrokerConfig returns a namespace specific Broker Configuration, and if
// that doesn't exist, return a Cluster Default and if that doesn't exist
// return an error.
func (d *Defaults) GetBrokerConfig(ns string) (*duckv1.KReference, error) {
	if d == nil {
		return nil, errors.New("Defaults are nil")
	}
	value, present := d.NamespaceDefaultsConfig[ns]
	if present {
		return value, nil
	}
	if d.ClusterDefault != nil {
		return d.ClusterDefault, nil
	}
	return nil, errors.New("Defaults for Broker Configurations have not been set up.")
}
