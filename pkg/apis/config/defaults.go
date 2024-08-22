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

/*
The priority precedence for determining the broker class and configuration is as follows:

1. If a specific broker class is provided:
   a. Check namespace-specific configuration for the given broker class
   b. If not found, check cluster-wide configuration for the given broker class
   c. If still not found, use the cluster-wide default configuration

2. If no specific broker class is provided:
   a. Check namespace-specific default broker class
   b. If found, use its configuration (following step 1)
   c. If not found, use cluster-wide default broker class and configuration

3. If no default cluster configuration is set, return an error

4. If no default namespace configuration is set, will use the cluster-wide default configuration.

This can be represented as a flow chart:

                    Start
                      |
                      v
          Is broker class provided?
                 /            \
               Yes            No
               /                \
   Check namespace config    Check namespace
   for provided class        default class
         |                        |
         v                        v
    Found?                    Found?
     /    \                    /    \
   Yes    No                 Yes    No
   |       |                 |       |
   |       |                 |       |
   |       v                 |       v
   |   Check cluster         |   Use cluster
   |   config for class      |   default class
   |       |                 |   and config
   |       v                 |
   |    Found?               |
   |    /    \               |
   |  Yes    No              |
   |   |      |              |
   |   |      v              |
   |   |   Use cluster       |
   |   |   default config    |
   v   v                     v
 Use found configuration

The system prioritizes namespace-specific configurations over cluster-wide defaults,
and explicitly provided broker classes over default classes.
*/

// Defaults includes the default values to be populated by the webhook.
type Defaults struct {
	// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
	// Namespace is the key, the value is the NamespaceDefaultConfig
	NamespaceDefaultsConfig map[string]*DefaultConfig `json:"namespaceDefaults,omitempty"`

	// ClusterDefaultConfig is the default broker config for all the namespaces that
	// are not in NamespaceDefaultBrokerConfigs. Different broker class could have
	// different default config.
	ClusterDefaultConfig *DefaultConfig `json:"clusterDefault,omitempty"`
}

// DefaultConfig struct contains the default configuration for the cluster and namespace.
type DefaultConfig struct {
	// DefaultBrokerClass and DefaultBrokerClassConfig are the default broker class for the whole cluster/namespace.
	// Users have to specify both of them
	DefaultBrokerClass string `json:"brokerClass,omitempty"`

	//Deprecated: Use the config in BrokerClasses instead
	*BrokerConfig `json:",inline"`

	// Optional: BrokerClasses are the default broker classes' config. The key is the broker class name, and the value is the config for that broker class.
	BrokerClasses map[string]*BrokerConfig `json:"brokerClasses,omitempty"`

	DisallowDifferentNamespaceConfig *bool `json:"disallowDifferentNamespaceConfig,omitempty"`
}

// BrokerConfig contains configuration for a given broker. Allows
// configuring the reference to the
// config it should use and it's delivery.
type BrokerConfig struct {
	*duckv1.KReference `json:",inline"`
	Delivery           *eventingduckv1.DeliverySpec `json:"delivery,omitempty"`
}

// GetBrokerConfig returns a namespace specific Broker Config, and if
// that doesn't exist, return a Cluster Default and if that doesn't exist return an error.
func (d *Defaults) GetBrokerConfig(ns string, brokerClassName *string) (*BrokerConfig, error) {
	if d == nil {
		return nil, errors.New("Defaults for Broker Configurations for cluster have not been set up. You can set them via ConfigMap config-br-defaults.")
	}

	// Early return if brokerClassName is provided and valid
	if brokerClassName != nil && *brokerClassName != "" {
		return d.getBrokerConfigByClassName(ns, *brokerClassName)
	}

	// Handling empty brokerClassName
	return d.getBrokerConfigForEmptyClassName(ns)
}

// getBrokerConfigByClassName returns the BrokerConfig for the given brokerClassName.
// It first checks the namespace specific configuration, then the cluster default configuration.
func (d *Defaults) getBrokerConfigByClassName(ns string, brokerClassName string) (*BrokerConfig, error) {
	// Check namespace specific configuration
	if nsConfig, ok := d.NamespaceDefaultsConfig[ns]; ok && nsConfig != nil {
		// check if the brokerClassName is the default broker class for this namespace
		if nsConfig.DefaultBrokerClass == brokerClassName {
			if nsConfig.BrokerConfig == nil {
				// as no default broker class config is set for this namespace, check whether nsconfig's brokerClasses map has the config for this broker class
				if config, ok := nsConfig.BrokerClasses[brokerClassName]; ok && config != nil {
					return config, nil
				}
				// if not found, return the cluster default config
				return d.getClusterDefaultBrokerConfig(brokerClassName)
			} else {
				// If the brokerClassName exists in the BrokerClasses, return the config in the BrokerClasses
				if config, ok := nsConfig.BrokerClasses[brokerClassName]; ok && config != nil {
					return config, nil
				} else {
					// If the brokerClassName is the default broker class for the namespace, return the BrokerConfig
					return nsConfig.BrokerConfig, nil
				}
			}
		} else {
			// if the brokerClassName is not the default broker class for the namespace, check whether nsconfig's brokerClasses map has the config for this broker class
			if config, ok := nsConfig.BrokerClasses[brokerClassName]; ok && config != nil {
				return config, nil
			}
		}
	}

	// Check cluster default configuration
	return d.getClusterDefaultBrokerConfig(brokerClassName)
}

// getBrokerConfigForEmptyClassName returns the BrokerConfig for the given namespace when brokerClassName is empty.
// It first checks the namespace specific configuration, then the cluster default configuration.
func (d *Defaults) getBrokerConfigForEmptyClassName(ns string) (*BrokerConfig, error) {
	// Check if namespace has a default broker class
	if nsConfig, ok := d.NamespaceDefaultsConfig[ns]; ok && nsConfig != nil {
		if nsConfig.DefaultBrokerClass != "" {
			return d.getBrokerConfigByClassName(ns, nsConfig.DefaultBrokerClass)
		}
	}

	// Fallback to cluster default configuration
	return d.getClusterDefaultBrokerConfig("")
}

// getClusterDefaultBrokerConfig returns the BrokerConfig for the given brokerClassName.
func (d *Defaults) getClusterDefaultBrokerConfig(brokerClassName string) (*BrokerConfig, error) {
	if d.ClusterDefaultConfig == nil || d.ClusterDefaultConfig.BrokerConfig == nil {
		return nil, errors.New("Defaults for Broker Configurations for cluster have not been set up. You can set them via ConfigMap config-br-defaults.")
	}

	// Check if the brokerClassName is the default broker class for the whole cluster
	if brokerClassName == "" || d.ClusterDefaultConfig.DefaultBrokerClass == brokerClassName {
		// If the brokerClassName exists in the BrokerClasses, return the config in the BrokerClasses
		if config, ok := d.ClusterDefaultConfig.BrokerClasses[brokerClassName]; ok && config != nil {
			return config, nil
		} else {
			// If the brokerClassName is the default broker class for the cluster, return the BrokerConfig
			return d.ClusterDefaultConfig.BrokerConfig, nil
		}
	}

	if config, ok := d.ClusterDefaultConfig.BrokerClasses[brokerClassName]; ok && config != nil {
		return config, nil
	}

	return d.ClusterDefaultConfig.BrokerConfig, nil
}

// GetBrokerClass returns a namespace specific Broker Class, and if
// that doesn't exist, return a Cluster Default and if the defaults doesn't exist
// return an error.
func (d *Defaults) GetBrokerClass(ns string) (string, error) {
	if d == nil {
		return "", errors.New("Defaults for Broker Configurations for cluster have not been set up. You can set them via ConfigMap config-br-defaults.")
	}

	// Check if the namespace has a specific configuration
	if nsConfig, ok := d.NamespaceDefaultsConfig[ns]; ok && nsConfig != nil {
		if nsConfig.DefaultBrokerClass != "" {
			return nsConfig.DefaultBrokerClass, nil
		}
	}

	// Fallback to cluster default configuration if namespace specific configuration is not set
	if d.ClusterDefaultConfig != nil && d.ClusterDefaultConfig.DefaultBrokerClass != "" {
		return d.ClusterDefaultConfig.DefaultBrokerClass, nil
	}

	// If neither namespace specific nor cluster default broker class is found
	return "", fmt.Errorf("Neither namespace specific nor cluster default broker class is found for namespace %q, please set them via ConfigMap config-br-defaults", ns)
}
