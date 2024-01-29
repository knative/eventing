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

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"

	"sigs.k8s.io/yaml"

	corev1 "k8s.io/api/core/v1"

	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	// DefaultsConfigName is the name of config map for the default
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
	return json.Unmarshal(j, &out)
}

// NewDefaultsConfigFromConfigMap creates a Defaults from the supplied configMap
func NewDefaultsConfigFromConfigMap(config *corev1.ConfigMap) (*Defaults, error) {
	return NewDefaultsConfigFromMap(config.Data)
}

// Defaults includes the default values to be populated by the webhook.
type Defaults struct {
	// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
	// Namespace is the key, the value is the NamespaceDefaultConfig
	NamespaceDefaultsConfig *NamespaceDefaultsConfig `json:"namespaceDefaults,omitempty"`

	// ClusterDefaultConfig is the default broker config for all the namespaces that
	// are not in NamespaceDefaultBrokerConfigs. Different broker class could have
	// different default config.
	ClusterDefaultConfig *DefaultConfig `json:"clusterDefault,omitempty"`
}

// DefaultConfig struct contains the default configuration for the cluster and namespace.
type DefaultConfig struct {
	// DefaultBrokerClass and DefaultBrokerClassConfig are the default broker class for the whole cluster
	// Users have to specify both of them
	DefaultBrokerClass       string        `json:"brokerClass,omitempty"`
	DefaultBrokerClassConfig *BrokerConfig `json:",inline"`

	// BrokerClasses are the default broker classes' config for the whole cluster
	BrokerClasses map[string]*BrokerConfig `json:"brokerClasses,omitempty"`

	DisallowDifferentNamespaceConfig *bool `json:"disallowDifferentNamespaceConfig,omitempty"`
}

// NamespaceDefaultsConfig struct contains the default configuration for the namespace.
type NamespaceDefaultsConfig struct {

	// BrokerClasses are the default broker classes' config for the namespace
	NameSpaces map[string]*DefaultConfig `json:"brokerClasses,omitempty"`

	DisallowDifferentNamespaceConfig *bool `json:"disallowDifferentNamespaceConfig,omitempty"`
}

// BrokerConfig contains configuration for a given namespace for broker. Allows
// configuring the reference to the
// config it should use and it's delivery.
type BrokerConfig struct {
	*duckv1.KReference `json:",inline"`
	Delivery           *eventingduckv1.DeliverySpec `json:"delivery,omitempty"`
}

// GetBrokerConfig returns a namespace specific Broker Configuration, and if
// that doesn't exist, return a Cluster Default and if that doesn't exist
// return an error.
func (d *Defaults) GetBrokerConfig(ns string, brokerClassName string) (*BrokerConfig, error) {

	if d == nil {
		return nil, errors.New("Defaults are nil")
	}

	// Check if the brokerClassName is empty
	if brokerClassName == "" {
		// We don't have the brokerClassName, check if we have the namespace default config
		if d.NamespaceDefaultsConfig != nil {
			// We have the namespace default config, check if we have the default broker class config for this namespace
			value, present := d.NamespaceDefaultsConfig.NameSpaces[ns]
			if present && value != nil {
				// We have the default broker class config for this namespace, return the config
				brokerClassName = value.DefaultBrokerClass
			} else {
				// We don't have the default broker class config for this namespace, check if we have the cluster default config
				if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClassConfig != nil {
					brokerClassName = d.ClusterDefaultConfig.DefaultBrokerClass
				} else {
					return nil, errors.New("Broker class name is empty and Defaults for Broker Configurations have not been set up. Cannot proceed further.")
				}
			}
		} else {
			// We don't have the namespace default config, check if we have the cluster default config
			if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClassConfig != nil {
				return d.ClusterDefaultConfig.DefaultBrokerClassConfig, nil
			} else {
				return nil, errors.New("Defaults for Broker Configurations have not been set up.")
			}
		}
	}

	value, present := d.NamespaceDefaultsConfig.NameSpaces[ns]
	if present && value != nil {
		// We have the namespace specific config
		// Now check whether the brokerClassName is the default broker class for this namespace
		if value.DefaultBrokerClass == brokerClassName {
			// return the default broker class config for this namespace
			return value.DefaultBrokerClassConfig, nil
		} else {
			// If the brokerClassName is not the default broker class for this namespace, check if we have the config for this brokerclass
			if value.BrokerClasses != nil {
				brokerConfig, present := value.BrokerClasses[brokerClassName]
				if present && brokerConfig != nil {
					// We have the config specifically for this broker class in this namespace, just return the config
					return brokerConfig, nil
				} else {
					// We don't have the config specifically for this broker class in this namespace, check the default broker class config for this namespace
					if value.DefaultBrokerClassConfig != nil {
						// return the default broker class config for this namespace
						return value.DefaultBrokerClassConfig, nil
					} else {
						// We don't have the default broker class config for this namespace, check if we have the cluster default config
						if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClassConfig != nil {
							return d.ClusterDefaultConfig.DefaultBrokerClassConfig, nil
						} else {
							return nil, errors.New("Defaults for Broker Configurations have not been set up.")
						}
					}
				}

			} else {
				// No brokerClasses config for this namespace, check the default broker class config for this namespace
				if value.DefaultBrokerClassConfig != nil {
					// return the default broker class config for this namespace
					return value.DefaultBrokerClassConfig, nil
				} else {
					// We don't have the default broker class config for this namespace, check if we have the cluster default config
					if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClassConfig != nil {
						return d.ClusterDefaultConfig.DefaultBrokerClassConfig, nil
					} else {
						return nil, errors.New("Defaults for Broker Configurations have not been set up. Both default broker class config for this namespace and cluster default config are nil.")
					}
				}
			}
		}
	}
	if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClassConfig != nil {
		return d.ClusterDefaultConfig.DefaultBrokerClassConfig, nil
	}
	return nil, errors.New("The given broker class name cannout be found in the namespace defaults config or cluster default config.")

}

// GetBrokerClass returns a namespace specific Broker Class, and if
// that doesn't exist, return a Cluster Default and if that doesn't exist
// return an error.
func (d *Defaults) GetBrokerClass(ns string) (string, error) {

	if d == nil {
		return "", errors.New("Defaults are nil")
	}

	value, present := d.NamespaceDefaultsConfig.NameSpaces[ns]
	if present && value != nil {
		// We have the namespace specific config
		// Now check whether the brokerClassName is the default broker class for this namespace
		if value.DefaultBrokerClass != "" {
			// return the default broker class for this namespace
			return value.DefaultBrokerClass, nil
		} else {
			// If the brokerClassName is not set for this namespace, we will check for the cluster default config
			if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClass != "" {
				return d.ClusterDefaultConfig.DefaultBrokerClass, nil
			} else {
				return "", errors.New("Defaults for Broker Configurations have not been set up.")
			}
		}
	} else {
		// We don't have the namespace specific config, check for the cluster default config
		if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClass != "" {
			return d.ClusterDefaultConfig.DefaultBrokerClass, nil
		} else {
			return "", errors.New("Defaults for Broker Configurations have not been set up.")
		}
	}
}
