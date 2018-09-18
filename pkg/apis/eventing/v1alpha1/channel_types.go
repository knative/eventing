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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is an abstract resource that implements the Subscribable and Sinkable
// contracts. The Provisioner provisions infrastructure to accepts events and
// deliver to Subscriptions.
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

// Check that ConfigurationStatus may have its conditions managed.
var _ duckv1alpha1.ConditionsAccessor = (*ChannelStatus)(nil)

// Check that Channel implements the Conditions duck type.
var _ = duck.VerifyType(&Channel{}, &duckv1alpha1.Conditions{})

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

	// Subscribers is a list of the Subscribers to this channel. This is filled in
	// by the Subscriptions controller. Users should not mutate this field.
	Subscribers []ChannelSubscriberSpec `json:"subscribers,omitempty"`
}

// ChannelSubscriberSpec defines a single subscriber to a Channel. At least one
// of Call or Result must be present.
type ChannelSubscriberSpec struct {
	// Call is an optional reference to a function for processing events.
	// Events from the From channel will be delivered here and replies
	// are optionally handled by Result.
	// +optional
	Call *Callable `json:"call,omitempty"`

	// Result optionally specifies how to handle events received from the Call
	// target.
	// +optional
	Result *ResultStrategy `json:"result,omitempty"`
}

var chanCondSet = duckv1alpha1.NewLivingConditionSet(ChannelConditionProvisioned)

// ChannelStatus represents the current state of a Channel.
type ChannelStatus struct {
	// ObservedGeneration is the most recent generation observed for this Channel.
	// It corresponds to the Channel's generation, which is updated on mutation by
	// the API Server.
	// TODO: The above comment is only true once
	// https://github.com/kubernetes/kubernetes/issues/58778 is fixed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DomainInternal holds the top-level domain that will distribute traffic
	// over the provided targets from inside the cluster. It generally has the
	// form {channel}.{namespace}.svc.cluster.local
	// TODO: move this to a struct that can be embedded similar to ObjectMeta and
	// TypeMeta.
	// +optional
	DomainInternal string `json:"domainInternal,omitempty"`

	// Represents the latest available observations of a channel's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

const (
	// ChannelConditionReady has status True when the Channel is ready to accept
	// traffic.
	ChannelConditionReady = duckv1alpha1.ConditionReady

	// ChannelConditionProvisioned has status True when the Channel's backing
	// resources have been provisioned.
	ChannelConditionProvisioned duckv1alpha1.ConditionType = "Provisioned"
)

func (c *Channel) GetSpecJSON() ([]byte, error) {
	return json.Marshal(c.Spec)
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (cs *ChannelStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return chanCondSet.Manage(cs).GetCondition(t)
}

// GetConditions returns the Conditions array. This enables generic handling of
// conditions by implementing the duckv1alpha1.Conditions interface.
func (cs *ChannelStatus) GetConditions() duckv1alpha1.Conditions {
	return cs.Conditions
}

// SetConditions sets the Conditions array. This enables generic handling of
// conditions by implementing the duckv1alpha1.Conditions interface.
func (cs *ChannelStatus) SetConditions(conditions duckv1alpha1.Conditions) {
	cs.Conditions = conditions
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a collection of Channels.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}
