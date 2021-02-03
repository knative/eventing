/*
Copyright 2021 The Knative Authors

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

package mtbroker

import (
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/configmap"
	"sigs.k8s.io/yaml"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	InternalDeliveryConfigMapName        = "mt-broker-internal-delivery"
	internalDeliveryConfigMapDeliveryKey = "delivery"
)

// NewInternalDeliveryConfigFromConfigMap parses the config map into a DeliverySpec
func NewInternalDeliveryConfigFromConfigMap(cm *corev1.ConfigMap) (*eventingduck.DeliverySpec, error) {
	if cm == nil || len(cm.Data) == 0 {
		return nil, nil
	}
	d, ok := cm.Data[internalDeliveryConfigMapDeliveryKey]
	if !ok {
		return nil, nil
	}

	var delivery eventingduck.DeliverySpec
	err := yaml.Unmarshal([]byte(d), &delivery)
	if err != nil {
		return nil, err
	}
	return &delivery, nil
}

// InternalDeliveryConfigStore is a typed wrapper around configmap.Untyped store to handle our configmaps.
// +k8s:deepcopy-gen=false
type InternalDeliveryConfigStore struct {
	*configmap.UntypedStore
}

// NewInternalDeliveryConfigStore creates a new store of Configs and optionally calls functions when ConfigMaps are updated.
func NewInternalDeliveryConfigStore(logger configmap.Logger, onAfterStore ...func(name string, value interface{})) *InternalDeliveryConfigStore {
	store := &InternalDeliveryConfigStore{
		UntypedStore: configmap.NewUntypedStore(
			"channeldefaults",
			logger,
			configmap.Constructors{
				InternalDeliveryConfigMapName: NewInternalDeliveryConfigFromConfigMap,
			},
			onAfterStore...,
		),
	}

	return store
}

// Load creates a InternalDeliveryConfigStore from the current config state of the InternalDeliveryConfigStore.
func (s *InternalDeliveryConfigStore) Load() *eventingduck.DeliverySpec {
	return s.UntypedLoad(InternalDeliveryConfigMapName).(*eventingduck.DeliverySpec)
}
