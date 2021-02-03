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
