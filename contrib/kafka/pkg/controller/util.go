package controller

import (
	"fmt"
	"log"
	"strings"

	cluster "github.com/bsm/sarama-cluster"

	"github.com/knative/pkg/configmap"
)

const (
	BrokerConfigMapKey                 = "bootstrap_servers"
	ConsumerModeConfigMapKey           = "consumer_mode"
	ConsumerModePartitionConsumerValue = "partitions"
	ConsumerModeMultiplexConsumerValue = "multiplex"
	KafkaChannelSeparator              = "."
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
	} else {
		return nil, fmt.Errorf("missing key %s in provisioner configuration", BrokerConfigMapKey)
	}

	config.ConsumerMode = cluster.ConsumerModeMultiplex
	if mode, ok := configMap[ConsumerModeConfigMapKey]; ok {
		switch strings.ToLower(mode) {
		case ConsumerModeMultiplexConsumerValue:
			config.ConsumerMode = cluster.ConsumerModeMultiplex
		case ConsumerModePartitionConsumerValue:
			config.ConsumerMode = cluster.ConsumerModePartitions
		default:
			log.Printf("consumer_mode: %q is invalid. Using default mode %q", mode, ConsumerModeMultiplexConsumerValue)
			config.ConsumerMode = cluster.ConsumerModeMultiplex
		}
	}
	return config, nil
}
