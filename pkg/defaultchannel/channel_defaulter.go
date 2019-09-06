/*
Copyright 2018 The Knative Authors

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

package defaultchannel

import (
	"encoding/json"
	"sync/atomic"

	"github.com/ghodss/yaml"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
)

const (
	// ConfigMapName is the name of the ConfigMap that contains the configuration for the default
	// channel CRD.
	ConfigMapName = "default-ch-webhook"

	// ChannelDefaulterKey is the key in the ConfigMap to get the name of the default
	// Channel CRD.
	ChannelDefaulterKey = "default-ch-config"
)

// Config is the data structure serialized to YAML in the config map. When a Channel needs to be
// defaulted, the Channel's namespace will be used as a key into NamespaceDefaults, if there is
// something present, then that is used. If not, then the ClusterDefault is used.
type Config struct {
	// NamespaceDefaultChannels are the default Channels CRDs for each namespace. namespace is the
	// key, the value is the default ChannelTemplate to use.
	NamespaceDefaults map[string]*eventingduckv1alpha1.ChannelTemplateSpec `json:"namespaceDefaults,omitempty"`
	// ClusterDefaultChannel is the default Channel CRD for all namespaces that are not in
	// NamespaceDefaultChannels.
	ClusterDefault *eventingduckv1alpha1.ChannelTemplateSpec `json:"clusterDefault,omitempty"`
}

// ChannelDefaulter adds a default Channel CRD to Channels that do not have any
// CRD specified. The default is stored in a ConfigMap and can be updated at runtime.
type ChannelDefaulter struct {
	// The current default Channel CRD to set. This should only be accessed via
	// getConfig() and setConfig(), as they correctly enforce the type we require (*Config).
	config atomic.Value
	logger *zap.Logger
}

var _ eventingduckv1alpha1.ChannelDefaulter = &ChannelDefaulter{}

// New creates a new ChannelDefaulter. The caller is expected to set this as the global singleton.
//
// channelDefaulter := channeldefaulter.New(logger)
// messagingv1alpha1.ChannelDefaulterSingleton = channelDefaulter
// configMapWatcher.Watch(channelDefaulter.ConfigMapName, channelDefaulter.UpdateConfigMap)
func New(logger *zap.Logger) *ChannelDefaulter {
	return &ChannelDefaulter{
		logger: logger.With(zap.String("role", "channelDefaulter")),
	}
}

// UpdateConfigMap reads in a ConfigMap and updates the internal default Channel CRD to use.
func (cd *ChannelDefaulter) UpdateConfigMap(cm *corev1.ConfigMap) {
	if cm == nil {
		cd.logger.Info("UpdateConfigMap on a nil map")
		return
	}
	defaultChannelConfig, present := cm.Data[ChannelDefaulterKey]
	if !present {
		cd.logger.Info("ConfigMap is missing key", zap.String("key", ChannelDefaulterKey), zap.Any("configMap", cm))
		return
	}

	if defaultChannelConfig == "" {
		cd.logger.Info("ConfigMap's value was the empty string, ignoring it.", zap.Any("configMap", cm))
		return
	}

	defaultChannelConfigJson, err := yaml.YAMLToJSON([]byte(defaultChannelConfig))
	if err != nil {
		cd.logger.Error("ConfigMap's value could not be converted to JSON.", zap.Error(err), zap.String("defaultChannelConfig", defaultChannelConfig))
		return
	}

	config := &Config{}
	if err := json.Unmarshal([]byte(defaultChannelConfigJson), config); err != nil {
		cd.logger.Error("ConfigMap's value could not be unmarshaled.", zap.Error(err), zap.Any("configMap", cm))
		return
	}

	cd.logger.Info("Updated channelDefaulter config", zap.Any("config", config))
	cd.setConfig(config)
}

// setConfig is a typed wrapper around config.
func (cd *ChannelDefaulter) setConfig(config *Config) {
	cd.config.Store(config)
}

// getConfig is a typed wrapper around config.
func (cd *ChannelDefaulter) getConfig() *Config {
	if config, ok := cd.config.Load().(*Config); ok {
		return config
	}
	return nil
}

// GetDefault determines the default Channel CRD and arguments for the provided namespace. If there is no default
// for the provided namespace, then use the cluster default.
func (cd *ChannelDefaulter) GetDefault(namespace string) *eventingduckv1alpha1.ChannelTemplateSpec {
	// Because we are treating this as a singleton, be tolerant to it having not been setup at all.
	if cd == nil {
		return nil
	}
	config := cd.getConfig()
	if config == nil {
		return nil
	}
	channelTemplate := getDefaultChannelTemplate(config, namespace)
	cd.logger.Debug("Defaulting the Channel", zap.Any("defaultChannelTemplate", channelTemplate))
	return channelTemplate
}

func getDefaultChannelTemplate(config *Config, namespace string) *eventingduckv1alpha1.ChannelTemplateSpec {
	if template, ok := config.NamespaceDefaults[namespace]; ok {
		return template
	}
	return config.ClusterDefault
}
