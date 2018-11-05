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

package v1alpha1

import (
	"sync/atomic"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

const (
	// ChannelDefaulterConfigMapName is the name of the ConfigMap that contains the configuration
	// for the default ClusterChannelProvisioner.
	ChannelDefaulterConfigMapName = "default-channel-webhook"

	// channelDefaulterKey ey is the key in the ConfigMap to get the name of the default
	// ClusterChannelProvisioner.
	channelDefaulterKey = "default-provisioner-name"
)

var (
	// ChannelDefaulterSingleton is the singleton for applying the default ClusterChannelProvisioner
	// to all Channels that do not have a provisioner specified. It is setup by main():
	//
	// eventingv1alpha1.ChannelDefaulterSingleton = eventingv1alpha1.NewChannelDefaulter(logger)
	ChannelDefaulterSingleton *ChannelDefaulter
)

// ChannelDefaulter adds a default ClusterChannelProvisioner to Channels that do not have any
// provisioner specified. The default is stored in a ConfigMap and can be updated at runtime.
type ChannelDefaulter struct {
	// The current default ClusterChannelProvisioner to set. This should only be accessed via
	// getConfig() and setConfig(), as they correctly enforce the type we require
	// (*corev1.ObjectReference).
	config atomic.Value
	logger *zap.Logger
}

// NewChannelDefaulter creates a new ChannelDefaulter. The caller is expected to set this as the
// global singleton:
//
// eventingv1alpha1.ChannelDefaulterSingleton = eventingv1alpha1.NewChannelDefaulter(logger)
func NewChannelDefaulter(logger *zap.Logger) *ChannelDefaulter {
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
	defaultProvisionerName, present := cm.Data[channelDefaulterKey]
	if !present {
		cd.logger.Info("ConfigMap is missing key", zap.String("key", channelDefaulterKey), zap.Any("configMap", cm))
		return
	}
	if defaultProvisionerName == "" {
		cd.logger.Info("ConfigMap's value is the empty string", zap.String("key", channelDefaulterKey), zap.Any("configMap", cm))
		return
	}

	config := &corev1.ObjectReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "ClusterChannelProvisioner",
		Name:       defaultProvisionerName,
	}
	cd.setConfig(config)
}

// setConfig is a typed wrapper around config.
func (cd *ChannelDefaulter) setConfig(provisioner *corev1.ObjectReference) {
	cd.config.Store(provisioner)
}

// getConfig is a typed wrapper around config.
func (cd *ChannelDefaulter) getConfig() *corev1.ObjectReference {
	return cd.config.Load().(*corev1.ObjectReference)
}

// setDefaultProvisioner sets the provisioner of the provided channel to the current default
// ClusterChannelProvisioner.
func (cd *ChannelDefaulter) setDefaultProvisioner(c *ChannelSpec) {
	// Because we are treating this as a singleton, be tolerant to it having not been setup at all.
	if cd == nil {
		return
	}
	// TODO Don't use a single default, instead use the Channel's arguments to determine the type of
	// Channel to use (e.g. it can say whether it needs to be persistent, strictly ordered, etc.).
	dp := cd.getConfig()
	cd.logger.Info("Defaulting the ClusterChannelProvisioner", zap.Any("defaultClusterChannelProvisioner", dp))
	c.Provisioner = dp
}
