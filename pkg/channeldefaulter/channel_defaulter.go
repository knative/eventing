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

package channeldefaulter

import (
	"sync/atomic"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ConfigMapName is the name of the ConfigMap that contains the configuration for the default
	// ClusterChannelProvisioner.
	ConfigMapName = "default-channel-webhook"

	// channelDefaulterKey is the key in the ConfigMap to get the name of the default
	// ClusterChannelProvisioner.
	channelDefaulterKey = "default-channel-config"
)

// Config is the data structure serialized to YAML in the config map. When a Channel needs to be
// defaulted, the Channel's namespace will be used as a key into NamespaceDefaults, if there is
// something present, then that is used. If not, then the ClusterDefault is used.
type Config struct {
	// NamespaceDefaults are the default Channel provisioner for each namespace. namespace is the
	// key, the value is the default provisioner to use.
	NamespaceDefaults map[string]*corev1.ObjectReference `json:"namespaceDefaults,omitempty"`
	// ClusterDefault is the default Channel provisioner for all namespaces that are not in
	// NamespaceDefaults.
	ClusterDefault *corev1.ObjectReference `json:"clusterDefault,omitempty"`
}

// ChannelDefaulter adds a default ClusterChannelProvisioner to Channels that do not have any
// provisioner specified. The default is stored in a ConfigMap and can be updated at runtime.
type ChannelDefaulter struct {
	// The current default ClusterChannelProvisioner to set. This should only be accessed via
	// getConfig() and setConfig(), as they correctly enforce the type we require (*Config).
	config atomic.Value
	logger *zap.Logger
}

var _ eventingv1alpha1.ChannelProvisionerDefaulter = &ChannelDefaulter{}

// New creates a new ChannelDefaulter. The caller is expected to set this as the global singleton.
//
// channelDefaulter := channeldefaulter.New(logger)
// eventingv1alpha1.ChannelDefaulterSingleton = channelDefaulter
// configMapWatcher.Watch(channeldefaulter.ConfigMapName, channelDefaulter.UpdateConfigMap)
func New(logger *zap.Logger) *ChannelDefaulter {
	return &ChannelDefaulter{
		logger: logger.With(zap.String("role", "channelDefaulter")),
	}
}

// UpdateConfigMap reads in a ConfigMap and updates the internal default ClusterChannelProvisioner
// to use.
func (cd *ChannelDefaulter) UpdateConfigMap(cm *corev1.ConfigMap) {
	if cm == nil {
		cd.logger.Info("UpdateConfigMap on a nil map")
		return
	}
	defaultChannelConfig, present := cm.Data[channelDefaulterKey]
	if !present {
		cd.logger.Info("ConfigMap is missing key", zap.String("key", channelDefaulterKey), zap.Any("configMap", cm))
		return
	}

	if defaultChannelConfig == "" {
		cd.logger.Info("ConfigMap's value was the empty string, ignoring it.", zap.Any("configMap", cm))
		return
	}

	config := Config{}
	if err := yaml.UnmarshalStrict([]byte(defaultChannelConfig), &config); err != nil {
		cd.logger.Error("ConfigMap's value could not be unmarshaled.", zap.Error(err), zap.Any("configMap", cm))
		return
	}

	cd.logger.Info("Updated channelDefaulter config", zap.Any("config", config))
	cd.setConfig(&config)
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

// GetDefault determines the default provisioner and arguments for the provided channel.
func (cd *ChannelDefaulter) GetDefault(c *eventingv1alpha1.Channel) (*corev1.ObjectReference, *runtime.RawExtension) {
	// Because we are treating this as a singleton, be tolerant to it having not been setup at all.
	if cd == nil {
		return nil, nil
	}
	if c == nil {
		return nil, nil
	}
	config := cd.getConfig()
	if config == nil {
		return nil, nil
	}

	// TODO Don't use a single default, instead use the Channel's arguments to determine the type of
	// Channel to use (e.g. it can say whether it needs to be persistent, strictly ordered, etc.).
	dp := getDefaultProvisioner(config, c.Namespace)
	cd.logger.Info("Defaulting the ClusterChannelProvisioner", zap.Any("defaultClusterChannelProvisioner", dp))
	return dp, nil
}

func getDefaultProvisioner(config *Config, namespace string) *corev1.ObjectReference {
	if dp, ok := config.NamespaceDefaults[namespace]; ok {
		return dp
	}
	return config.ClusterDefault
}
