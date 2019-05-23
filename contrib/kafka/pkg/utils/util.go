/*
Copyright 2019 The Knative Authors

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

package utils

import (
	"fmt"
	"log"
	"strings"

	"github.com/bsm/sarama-cluster"

	"github.com/knative/pkg/configmap"
)

const (
	BrokerConfigMapKey                 = "bootstrap_servers"
	ConsumerModeConfigMapKey           = "consumer_mode"
	ConsumerModePartitionConsumerValue = "partitions"
	ConsumerModeMultiplexConsumerValue = "multiplex"
	KafkaChannelSeparator              = "."

	// DefaultNumPartitions defines the default number of partitions
	DefaultNumPartitions = 1

	// DefaultReplicationFactor defines the default number of replications
	DefaultReplicationFactor = 1

	knativeKafkaTopicPrefix = "knative-messaging-kafka"
)

type KafkaConfig struct {
	Brokers      []string
	ConsumerMode cluster.ConsumerMode
}

// GetKafkaConfig returns the details of the Kafka cluster.
func GetKafkaConfig(path string) (*KafkaConfig, error) {
	configMap, err := configmap.Load(path)
	if err != nil {
		return nil, fmt.Errorf("error loading configuration: %s", err)
	}

	if len(configMap) == 0 {
		return nil, fmt.Errorf("missing configuration")
	}

	config := &KafkaConfig{}

	if brokers, ok := configMap[BrokerConfigMapKey]; ok {
		bootstrapServers := strings.Split(brokers, ",")
		for _, s := range bootstrapServers {
			if len(s) == 0 {
				return nil, fmt.Errorf("empty %s value in configuration", BrokerConfigMapKey)
			}
		}
		config.Brokers = bootstrapServers
	} else {
		return nil, fmt.Errorf("missing key %s in configuration", BrokerConfigMapKey)
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

func TopicName(separator, namespace, name string) string {
	topic := []string{knativeKafkaTopicPrefix, namespace, name}
	return strings.Join(topic, separator)
}
