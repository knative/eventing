package controller

import (
	"fmt"
	"strings"

	"github.com/knative/pkg/configmap"
)

const (
	BrokerConfigMapKey    = "bootstrap_servers"
	KafkaChannelSeparator = "."
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

	if brokers, ok := configMap[BrokerConfigMapKey]; ok {
		bootstrapServers := strings.Split(brokers, ",")
		for _, s := range bootstrapServers {
			if len(s) == 0 {
				return nil, fmt.Errorf("empty %s value in provisioner configuration", BrokerConfigMapKey)
			}
		}
		config.Brokers = bootstrapServers
		return config, nil
	}

	return nil, fmt.Errorf("missing key %s in provisioner configuration", BrokerConfigMapKey)
}
