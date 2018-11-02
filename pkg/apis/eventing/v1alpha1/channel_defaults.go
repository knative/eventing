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
	"log"
	"sync/atomic"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

//TODO replace this with openapi defaults when
// https://github.com/kubernetes/features/issues/575 lands (scheduled for 1.13)
func (c *Channel) SetDefaults() {
	c.Spec.SetDefaults()
}

func (fs *ChannelSpec) SetDefaults() {
	log.Println("in channelspec.setdefaults")
	if fs.Provisioner == nil {
		log.Println("in channelspec.setdefaults provisioner nil")
		if dc := Singleton; dc != nil {
			log.Println("in channelspec.setdefaults singleton not nil")
			dc.SetDefaultProvisioner(fs)
		}
	}
}

const (
	ConfigMapName = "default-channel-webhook"

	key = "default-provisioner-name"
)

var (
	Singleton *ChannelDefaulter
)

type ChannelDefaulter struct {
	config atomic.Value
	logger *zap.Logger
}

func New(logger *zap.Logger) *ChannelDefaulter {
	return &ChannelDefaulter{
		logger: logger.With(zap.String("role", "channelDefaulter")),
	}
}

func (cd *ChannelDefaulter) UpdateConfigMap(cm *corev1.ConfigMap) {
	if cm == nil {
		cd.logger.Info("UpdateConfigMap on a nil map")
		return
	}
	defaultProvisionerName, present := cm.Data[key]
	if !present {
		cd.logger.Info("ConfigMap is missing key", zap.String("key", key), zap.Any("configMap", cm))
		return
	}
	if defaultProvisionerName == "" {
		cd.logger.Info("ConfigMap's value is the empty string", zap.String("key", key), zap.Any("configMap", cm))
		return
	}

	config := &corev1.ObjectReference{
		APIVersion: SchemeGroupVersion.String(),
		Kind:       "ClusterChannelProvisioner",
		Name:       defaultProvisionerName,
	}
	cd.setConfig(config)
}

func (cd *ChannelDefaulter) SetDefaultProvisioner(c *ChannelSpec) {
	log.Println("in channeldefaulter.setdefaultprovisioner")
	// Because we are treating this as a singleton, be tolerant to it having not been setup at all.
	if cd == nil {
		return
	}
	// TODO Don't use a single default, instead use the Channel's arguments to determine the type of
	// Channel to use (e.g. it can say whether it needs to be persistent, strictly ordered, etc.).
	c.Provisioner = cd.getConfig()
	log.Println("set default provisioner")
}

func (cd *ChannelDefaulter) setConfig(provisioner *corev1.ObjectReference) {
	cd.config.Store(provisioner)
}

func (cd *ChannelDefaulter) getConfig() *corev1.ObjectReference {
	return cd.config.Load().(*corev1.ObjectReference)
}
