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

package test

const (
	// DefaultBrokerName is the name of the Broker that is automatically created after the current namespace is labeled.
	DefaultBrokerName = "default"

	// InMemoryChannelProvisioner is the in-memory-channel provisioner.
	InMemoryChannelProvisioner = "in-memory-channel"
	// GCPPubSubChannelProvisioner is the gcp-pubsub provisioner, which is under contrib/gcppubsub.
	GCPPubSubChannelProvisioner = "gcp-pubsub"
	// KafkaChannelProvisioner is the kafka provisioner, which is under contrib/kafka.
	KafkaChannelProvisioner = "kafka"
	// NatssChannelProvisioner is the natss provisioner, which is under contrib/natss
	NatssChannelProvisioner = "natss"
)
