package reconciler

import cluster "github.com/bsm/sarama-cluster"

const (
	ConsumerModePartitionConsumerValue = "partitions"
	ConsumerModeMultiplexConsumerValue = "multiplex"

	// DefaultNumPartitions defines the default number of partitions
	DefaultNumPartitions = 1

	// DefaultReplicationFactor defines the default number of replications
	DefaultReplicationFactor = 1
)

type KafkaProvisionerConfig struct {
	Brokers      []string
	ConsumerMode cluster.ConsumerMode
}
