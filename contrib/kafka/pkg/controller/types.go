package controller

import (
	"crypto/tls"

	cluster "github.com/bsm/sarama-cluster"
)

type KafkaProvisionerConfig struct {
	Brokers      []string
	ConsumerMode cluster.ConsumerMode
	TlsConfig    *tls.Config
}
