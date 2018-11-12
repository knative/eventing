package controller

import (
	"fmt"
	"strings"

	"github.com/knative/pkg/configmap"
)

const (
	BrokerConfigMapKey = "bootstrap_servers"
)

// GetProvisionerConfig returns the details of the associated ClusterChannelProvisioner object
func GetProvisionerConfig(path string) (*KafkaProvisionerConfig, error) {
	configMap, err := configmap.Load(path)
	if err != nil {
		return nil, fmt.Errorf("error loading provisioner configuration: %s", err)
	}

	if len(configMap) == 0 {
		return nil, fmt.Errorf("missing provisioner configuration")
	}

	config := &KafkaProvisionerConfig{}

	if value, ok := configMap[BrokerConfigMapKey]; ok {
		bootstrapServers := strings.Split(value, ",")
		if len(bootstrapServers) == 0 {
			return nil, fmt.Errorf("missing kafka brokers in provisioner configuration")
		}
		config.Brokers = bootstrapServers
		return config, nil
	}

	return nil, fmt.Errorf("missing key %s in provisioner configuration", BrokerConfigMapKey)
}
