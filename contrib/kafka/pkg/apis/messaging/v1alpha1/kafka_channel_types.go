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

package v1alpha1

import (
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaChannel is a resource representing a Kafka Channel.
type KafkaChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec KafkaChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the KafkaChannel. This data may be out of
	// date.
	// +optional
	Status KafkaChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*KafkaChannel)(nil)
var _ apis.Defaultable = (*KafkaChannel)(nil)
var _ runtime.Object = (*KafkaChannel)(nil)
var _ webhook.GenericCRD = (*KafkaChannel)(nil)

// KafkaChannelSpec defines the specification for a KafkaChannel.
type KafkaChannelSpec struct {
	// NumPartitions is the number of partitions of a Kafka topic. By default, it is set to 1.
	NumPartitions int32 `json:"numPartitions"`

	// ReplicationFactor is the replication factor of a Kafka topic. By default, it is set to 1.
	ReplicationFactor int16 `json:"replicationFactor"`

	// KafkaChannel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

// KafkaChannelStatus represents the current state of a KafkaChannel.
type KafkaChannelStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// KafkaChannel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	Address duckv1alpha1.Addressable `json:"address,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaChannelList is a collection of KafkaChannels.
type KafkaChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaChannel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for KafkaChannels
func (c *KafkaChannel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("KafkaChannel")
}
