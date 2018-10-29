/*
Copyright 2018 The Knative Authors

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterChannelProvisioner encapsulates a provisioning strategy for the
// backing resources required to realize a particular resource type.
type ClusterChannelProvisioner struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the Types provisioned by this Provisioner.
	Spec ClusterChannelProvisionerSpec `json:"spec"`

	// Status is the current status of the Provisioner.
	// +optional
	Status ClusterChannelProvisionerStatus `json:"status,omitempty"`
}

// Check that ClusterChannelProvisioner can be validated and can be defaulted.
var _ apis.Validatable = (*ClusterChannelProvisioner)(nil)
var _ apis.Defaultable = (*ClusterChannelProvisioner)(nil)
var _ runtime.Object = (*ClusterChannelProvisioner)(nil)
var _ webhook.GenericCRD = (*ClusterChannelProvisioner)(nil)

// ClusterChannelProvisionerSpec is the spec for a ClusterChannelProvisioner resource.
type ClusterChannelProvisionerSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`
}

var ccProvCondSet = duckv1alpha1.NewLivingConditionSet()

// ClusterChannelProvisionerStatus is the status for a ClusterChannelProvisioner resource
type ClusterChannelProvisionerStatus struct {
	// Conditions holds the state of a cluster provisioner at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the 'Generation' of the ClusterChannelProvisioner that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

const (
	// ClusterChannelProvisionerConditionReady has status True when the Controller reconciling objects
	// controlled by it is ready to control them.
	ClusterChannelProvisionerConditionReady = duckv1alpha1.ConditionReady
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *ClusterChannelProvisionerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return ccProvCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *ClusterChannelProvisionerStatus) IsReady() bool {
	return ccProvCondSet.Manage(ps).IsHappy()
}

// MarkProvisionerNotReady sets the condition that the provisioner is not ready to provision backing resource.
func (ps *ClusterChannelProvisionerStatus) MarkNotReady(reason, messageFormat string, messageA ...interface{}) {
	ccProvCondSet.Manage(ps).MarkFalse(ClusterChannelProvisionerConditionReady, reason, messageFormat, messageA...)
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *ClusterChannelProvisionerStatus) InitializeConditions() {
	ccProvCondSet.Manage(ps).InitializeConditions()
}

// MarkReady marks this ClusterChannelProvisioner as Ready=true.
//
// Note that this is not the normal pattern for duck conditions, but because there is (currently)
// no other condition on ClusterChannelProvisioners, the normal IsReady() logic doesn't work well.
func (ps *ClusterChannelProvisionerStatus) MarkReady() {
	ccProvCondSet.Manage(ps).MarkTrue(ClusterChannelProvisionerConditionReady)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterChannelProvisionerList is a list of ClusterChannelProvisioner resources
type ClusterChannelProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterChannelProvisioner `json:"items"`
}
