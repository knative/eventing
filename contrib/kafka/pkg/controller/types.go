package controller

import cluster "github.com/bsm/sarama-cluster"

type KafkaProvisionerConfig struct {
	Brokers      []string
	ConsumerMode cluster.ConsumerMode
}
