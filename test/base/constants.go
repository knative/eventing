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

package base

import "github.com/knative/eventing/pkg/reconciler/namespace/resources"

const (
	// DefaultBrokerName is the name of the Broker that is automatically created after the current namespace is labeled.
	DefaultBrokerName = resources.DefaultBrokerName

	// InMemoryProvisioner is the in-memory provisioner, which is also the default one.
	InMemoryProvisioner = "in-memory"
	// GCPPubSubProvisioner is the gcp-pubsub provisioner, which is under contrib/gcppubsub.
	GCPPubSubProvisioner = "gcp-pubsub"
	// KafkaProvisioner is the kafka provisioner, which is under contrib/kafka.
	KafkaProvisioner = "kafka"
	// NatssProvisioner is the natss provisioner, which is under contrib/natss
	NatssProvisioner = "natss"
)

// API versions for the resources.
const (
	EventingAPIVersion  = "eventing.knative.dev/v1alpha1"
	SourcesAPIVersion   = "sources.eventing.knative.dev/v1alpha1"
	MessagingAPIVersion = "messaging.knative.dev/v1alpha1"
)

// kind for eventing resources.
const (
	// ChannelKind is the kind for Channel.
	ChannelKind string = "Channel"
	// SubscriptionKind is the kind for Subscription.
	SubscriptionKind string = "Subscription"
	// ClusterChannelProvisionerKind is the kind for ClusterChannelProvisioner.
	ClusterChannelProvisionerKind string = "ClusterChannelProvisioner"

	// BrokerKind is the kind for Broker.
	BrokerKind string = "Broker"
	// TriggerKind is the kind for Trigger.
	TriggerKind string = "Trigger"
)

// kind for messaging resources.
const (
	// InMemoryChannelKind is the kind for InMemoryChannel.
	InMemoryChannelKind string = "InMemoryChannel"
	// KafkaChannelKind is the kind for KafkaChannel.
	KafkaChannelKind string = "KafkaChannel"
	// NatssChannelKind     string = "NatssChannel"
	// GCPPubSubChannelKind string = "GCPPubSubChannel"
)

// kind for sources resources.
const (
	// CronJobSourceKind is the kind for CronJobSource.
	CronJobSourceKind string = "CronJobSource"
	// ContainerSourceKind is the kind for ContainerSource.
	ContainerSourceKind string = "ContainerSource"
	// ApiServerSourceKind is the kind for ApiServerSource.
	ApiServerSourceKind string = "ApiServerSource"
)
