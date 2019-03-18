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
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is an abstract resource that implements the Addressable contract.
// The Provisioner provisions infrastructure to accepts events and
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

// ChannelSpec specifies the Provisioner backing a channel and the configuration
// arguments for a Channel.
type ChannelSpec struct {
	// TODO By enabling the status subresource metadata.generation should increment
	// thus making this property obsolete.
	//
	// We should be able to drop this property with a CRD conversion webhook
	// in the future
	//
	// +optional
	DeprecatedGeneration int64 `json:"generation,omitempty"`

	// Provisioner defines the name of the Provisioner backing this channel.
	Provisioner *corev1.ObjectReference `json:"provisioner,omitempty"`

	// Arguments defines the arguments to pass to the Provisioner which
	// provisions this Channel.
	// +optional
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`

	// Channel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

var chanCondSet = duckv1alpha1.NewLivingConditionSet(ChannelConditionProvisioned, ChannelConditionAddressable, ChannelConditionProvisionerInstalled)

// ChannelStatus represents the current state of a Channel.
type ChannelStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// Channel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	Address duckv1alpha1.Addressable `json:"address,omitempty"`

	// Internal is status unique to each ClusterChannelProvisioner.
	// +optional
	Internal *runtime.RawExtension `json:"internal,omitempty"`
}

const (
	// ChannelConditionReady has status True when the Channel is ready to
	// accept traffic.
	ChannelConditionReady = duckv1alpha1.ConditionReady

	// ChannelConditionProvisioned has status True when the Channel's
	// backing resources have been provisioned.
	ChannelConditionProvisioned duckv1alpha1.ConditionType = "Provisioned"

	// ChannelConditionAddressable has status true when this Channel meets
	// the Addressable contract and has a non-empty hostname.
	ChannelConditionAddressable duckv1alpha1.ConditionType = "Addressable"

	// ChannelConditionProvisionerFound has status true when the channel is being watched
	// by the provisioner's channel controller (in other words, the provisioner is installed)
	ChannelConditionProvisionerInstalled duckv1alpha1.ConditionType = "ProvisionerInstalled"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (cs *ChannelStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return chanCondSet.Manage(cs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (cs *ChannelStatus) IsReady() bool {
	return chanCondSet.Manage(cs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (cs *ChannelStatus) InitializeConditions() {
	chanCondSet.Manage(cs).InitializeConditions()
	// Channel-default-controller sets ChannelConditionProvisionerInstalled=False, and it needs to be set to True by individual controllers
	// This is done so that each individual channel controller gets it for free.
	// It is also implied here that the channel-default-controller never calls InitializeConditions(), while individual channel controllers
	// call InitializeConditions() as one of the first things in its reconcile loop.
	cs.MarkProvisionerInstalled()
}

// MarkProvisioned sets ChannelConditionProvisioned condition to True state.
func (cs *ChannelStatus) MarkProvisioned() {
	chanCondSet.Manage(cs).MarkTrue(ChannelConditionProvisioned)
}

// MarkNotProvisioned sets ChannelConditionProvisioned condition to False state.
func (cs *ChannelStatus) MarkNotProvisioned(reason, messageFormat string, messageA ...interface{}) {
	chanCondSet.Manage(cs).MarkFalse(ChannelConditionProvisioned, reason, messageFormat, messageA...)
}

// MarkProvisionerInstalled sets ChannelConditionProvisionerInstalled condition to True state.
func (cs *ChannelStatus) MarkProvisionerInstalled() {
	chanCondSet.Manage(cs).MarkTrue(ChannelConditionProvisionerInstalled)
}

// MarkProvisionerNotInstalled sets ChannelConditionProvisionerInstalled condition to False state.
func (cs *ChannelStatus) MarkProvisionerNotInstalled(reason, messageFormat string, messageA ...interface{}) {
	chanCondSet.Manage(cs).MarkFalse(ChannelConditionProvisionerInstalled, reason, messageFormat, messageA...)
}

// SetAddress makes this Channel addressable by setting the hostname. It also
// sets the ChannelConditionAddressable to true.
func (cs *ChannelStatus) SetAddress(hostname string) {
	cs.Address.Hostname = hostname
	if hostname != "" {
		chanCondSet.Manage(cs).MarkTrue(ChannelConditionAddressable)
	} else {
		chanCondSet.Manage(cs).MarkFalse(ChannelConditionAddressable, "emptyHostname", "hostname is the empty string")
	}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a collection of Channels.
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Channel `json:"items"`
}
