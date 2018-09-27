package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	runtimeClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/knative/eventing/pkg/system"
)

const (
	// controllerConfigMapName is the name of the configmap in the eventing
	// namespace that holds the configuration for this controller.
	ControllerConfigMapName = "kafka-provisioner-config"

	ProvisionerNameConfigMapKey      = "provisioner-name"
	ProvisionerNamespaceConfigMapKey = "provisioner-namespace"
	BrokerConfigMapKey               = "brokers"
)

type KafkaProvisionerConfig struct {
	Name      string
	Namespace string
	Brokers   []string
}

// GetProvisionerConfig returns the details of the associated Provisioner/ClusterProvisioner object
func GetProvisionerConfig(client runtimeClient.Client) (*KafkaProvisionerConfig, error) {
	configMapKey := runtimeClient.ObjectKey{
		Namespace: system.Namespace,
		Name:      ControllerConfigMapName,
	}

	configMap := &corev1.ConfigMap{}
	if err := client.Get(context.TODO(), configMapKey, configMap); err != nil {
		return nil, err
	}

	config := &KafkaProvisionerConfig{}

	if value, ok := configMap.Data[ProvisionerNameConfigMapKey]; ok {
		config.Name = value
	} else {
		return nil, fmt.Errorf("missing key %s in config map %s", ProvisionerNameConfigMapKey, ControllerConfigMapName)
	}

	if value, ok := configMap.Data[ProvisionerNamespaceConfigMapKey]; ok {
		config.Namespace = value
	} else {
		return nil, fmt.Errorf("missing key %s in config map %s", ProvisionerNamespaceConfigMapKey, ControllerConfigMapName)
	}

	if value, ok := configMap.Data[BrokerConfigMapKey]; ok {
		brokers := strings.Split(value, ",")
		if len(brokers) == 0 {
			return nil, fmt.Errorf("missing kafka brokers in configmap %s", ControllerConfigMapName)
		}
		config.Brokers = brokers
		return config, nil
	}

	return nil, fmt.Errorf("missing key %s in config map %s", BrokerConfigMapKey, ControllerConfigMapName)
}
