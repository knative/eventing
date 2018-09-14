/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"encoding/json"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel represents a named endpoint on which a Bus accepts event delivery and
// corresponds to the channels.channels.knative.dev CRD. The Bus handles
// provisioning channels, delivering events to Channels, and delivering events
// from Channels to their Subscriptions.
type Channel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec ChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the Channel. This data may be out of
	// date.
	// +optional
	Status ChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Channel)(nil)
var _ apis.Defaultable = (*Channel)(nil)
var _ apis.Immutable = (*Channel)(nil)
var _ runtime.Object = (*Channel)(nil)
var _ webhook.GenericCRD = (*Channel)(nil)

// ChannelSpec specifies the Provisioner backing a channel and the configuration
// arguments for a Channel.
type ChannelSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Provisioner defines the name of the Provisioner backing this channel.
	// TODO: +optional If missing, a default Provisioner may be selected for the Channel.
	Provisioner *ProvisionerReference `json:"provisioner,omitempty"`

	// Arguments defines the arguments to pass to the Provisioner which provisions
	// this Channel.
	// +optional
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`

	//TODO Subscription spec array
}

// ChannelStatus represents the current state of a Channel.
type ChannelStatus struct {
	// ObservedGeneration is the most recent generation observed for this Channel.
	// It corresponds to the Channel's generation, which is updated on mutation by
	// the API Server.
	//TODO The above comment is only true once
	// https://github.com/kubernetes/kubernetes/issues/58778 is fixed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DomainInternal holds the top-level domain that will distribute traffic
	// over the provided targets from inside the cluster. It generally has the
	// form {channel}.{namespace}.svc.cluster.local
	//TODO move this to a struct that can be embedded similar to ObjectMeta and
	// TypeMeta.
	// +optional
	DomainInternal string `json:"domainInternal,omitempty"`

	// Represents the latest available observations of a channel's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ChannelCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

type ChannelConditionType string

const (
	// ChannelConditionReady has status True when the Channel is ready to accept
	// traffic.
	ChannelConditionReady ChannelConditionType = "Ready"

	// ChannelConditionProvisioned has status True when the Channel's backing
	// resources have been provisioned.
	ChannelConditionProvisioned ChannelConditionType = "Provisioned"
)

// ChannelCondition describes the state of a channel at a point in time.
type ChannelCondition struct {
	// Type of channel condition.
	Type ChannelConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status corev1.ConditionStatus `json:"status"`
	// LastTransitionTime from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Message is a human readable message indicating details about the
	// last transition.
	Message string `json:"message,omitempty"`
}

//TODO move this to ClusterProvisioner types
// ProvisionerReference defines the strategy for selecting a Provisioner for a
// provisioned resource.
type ProvisionerReference struct {
	// A reference to a specific Provisioner.
	//TODO: +optional add selector
	Ref *corev1.ObjectReference `json:"ref,omitempty"`
}

func (cs *ChannelStatus) GetCondition(t ChannelConditionType) *ChannelCondition {
	for _, cond := range cs.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

func (c *Channel) GetSpecJSON() ([]byte, error) {
	return json.Marshal(c.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a collection of Channels.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}
