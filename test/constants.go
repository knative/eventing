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
	// DefaultClusterChannelProvisioner is the default ClusterChannelProvisioner we will run tests against.
	DefaultClusterChannelProvisioner = InMemoryProvisioner

	// InMemoryChannelProvisioner is the in-memory-channel provisioner.
	// It will be delete in 0.7, see https://github.com/knative/eventing/pull/1062 for more info.
	InMemoryChannelProvisioner = "in-memory-channel"

	// InMemoryProvisioner is the in-memory provisioner, which is also the default one.
	InMemoryProvisioner = "in-memory"
	// GCPPubSubProvisioner is the gcp-pubsub provisioner, which is under contrib/gcppubsub.
	GCPPubSubProvisioner = "gcp-pubsub"
	// KafkaProvisioner is the kafka provisioner, which is under contrib/kafka.
	KafkaProvisioner = "kafka"
	// NatssProvisioner is the natss provisioner, which is under contrib/natss
	NatssProvisioner = "natss"
)

// validProvisioners is a list of provisioners that Eventing currently support.
var validProvisioners = []string{
	InMemoryChannelProvisioner,
	InMemoryProvisioner,
	GCPPubSubProvisioner,
	KafkaProvisioner,
	NatssProvisioner,
}
