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

package common

import "github.com/knative/eventing/pkg/reconciler/namespace/resources"

// ValidProvisionersMap saves the provisioner-features mapping.
// Each pair means the provisioner support the list of features.
var ValidProvisionersMap = map[string][]Feature{
	InMemoryProvisioner:  []Feature{FeatureBasic},
	GCPPubSubProvisioner: []Feature{FeatureBasic, FeatureRedelivery, FeaturePersistence},
	KafkaProvisioner:     []Feature{FeatureBasic, FeatureRedelivery, FeaturePersistence},
	NatssProvisioner:     []Feature{FeatureBasic, FeatureRedelivery, FeaturePersistence},
}

const (
	// DefaultBrokerName is the name of the Broker that is automatically created after the current namespace is labeled.
	DefaultBrokerName = resources.DefaultBrokerName
	// DefaultClusterChannelProvisioner is the default ClusterChannelProvisioner we will run tests against.
	DefaultClusterChannelProvisioner = InMemoryProvisioner

	// InMemoryProvisioner is the in-memory provisioner, which is also the default one.
	InMemoryProvisioner = "in-memory"
	// GCPPubSubProvisioner is the gcp-pubsub provisioner, which is under contrib/gcppubsub.
	GCPPubSubProvisioner = "gcp-pubsub"
	// KafkaProvisioner is the kafka provisioner, which is under contrib/kafka.
	KafkaProvisioner = "kafka"
	// NatssProvisioner is the natss provisioner, which is under contrib/natss
	NatssProvisioner = "natss"
)

// Feature is the feature supported by the Channel provisioner.
type Feature string

const (
	// FeatureBasic is the feature that should be supported by all Channel provisioners
	FeatureBasic Feature = "basic"
	// FeatureRedelivery means if downstream rejects an event, that request will be attempted again.
	FeatureRedelivery Feature = "redelivery"
	// FeaturePersistence means if Channel's Pod goes down, all events already ACKed by the Channel
	// will persist and be retransmitted when the Pod restarts.
	FeaturePersistence Feature = "persistence"
)
