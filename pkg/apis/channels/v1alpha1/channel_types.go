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
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Channel represents a named endpoint on which a Bus accepts event delivery and
// corresponds to the channels.channels.knative.dev CRD. The Bus handles
// provisioning channels, delivering events to Channels, and delivering events
// from Channels to their Subscriptions.
type Channel struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ChannelSpec   `json:"spec"`
	Status             ChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Channel)(nil)
var _ apis.Defaultable = (*Channel)(nil)
var _ apis.Immutable = (*Channel)(nil)
var _ runtime.Object = (*Channel)(nil)
var _ webhook.GenericCRD = (*Channel)(nil)

// ChannelSpec specifies the Bus backing a channel and the configuration
// arguments for the channel.
type ChannelSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Name of the bus backing this channel (optional)
	Bus string `json:"bus,omitempty"`

	// ClusterBus name of the clusterbus backing this channel (mutually exclusive with Bus)
	ClusterBus string `json:"clusterBus,omitempty"`

	// Arguments is a list of configuration arguments for the Channel. The
	// Arguments for a channel must contain values for each of the Parameters
	// specified by the Bus' spec.parameters.Channels field except the
	// Parameters that have a default value. If a Parameter has a default value
	// and it is not in the list of Arguments, the default value will be used; a
	// Parameter without a default value that does not have an Argument will
	// result in an error setting up the Channel.
	Arguments *[]Argument `json:"arguments,omitempty"`
}

type ChannelConditionType string

const (

	// Ready is set when all other conditions are met and the channel is ready to accept traffic.
	ChannelReady ChannelConditionType = "Ready"

	// Serviceable means the service addressing the channel exists.
	ChannelServiceable ChannelConditionType = "Serviceable"

	// Routable means the virtual service forwarding traffic from the channel service to the
	// bus is created.
	ChannelRoutable ChannelConditionType = "Routeable"

	// Provisioned means the channel backing construct on the bus middleware has been set up.
	ChannelProvisioned ChannelConditionType = "Provisioned"
)

// ChannelCondition describes the state of a channel at a point in time.
type ChannelCondition struct {
	// Type of channel condition.
	Type ChannelConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime meta_v1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime meta_v1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// ChannelStatus (computed) for a channel
type ChannelStatus struct {
	// A reference to the k8s Service backing this channel, if successfully synced.
	Service *v1.LocalObjectReference `json:"service,omitempty"`

	// A reference to the istio VirtualService backing this channel, if successfully synced.
	VirtualService *v1.LocalObjectReference `json:"virtualService,omitempty"`

	// Represents the latest available observations of a channel's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []ChannelCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// DomainInternal holds the top-level domain that will distribute traffic
	// over the provided targets from inside the cluster. It generally has the
	// form {channel}.{namespace}.svc.cluster.local
	// +optional
	DomainInternal string `json:"domainInternal,omitempty"`
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

// ChannelList returned in list operations
type ChannelList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Channel `json:"items"`
}
