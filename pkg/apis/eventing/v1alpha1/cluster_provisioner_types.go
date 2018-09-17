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

	"encoding/json"
	"github.com/knative/pkg/apis"
	duck "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
)

// +genclient
// +genclient:noStatus
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterProvisioner encapsulates a provisioning strategy for the backing
// resources required to realize a particular resource type.
type ClusterProvisioner struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the Types provisioned by this Provisioner.
	Spec ClusterProvisionerSpec `json:"spec"`

	// Status is the current status of the Provisioner.
	// +optional
	Status ClusterProvisionerStatus `json:"status,omitempty"`
}

// Check that ClusterProvisioner can be validated and can be defaulted.
var _ apis.Validatable = (*ClusterProvisioner)(nil)
var _ apis.Defaultable = (*ClusterProvisioner)(nil)
var _ runtime.Object = (*ClusterProvisioner)(nil)
var _ webhook.GenericCRD = (*ClusterProvisioner)(nil)

// Check that ConfigurationStatus may have its conditions managed.
var _ duck.ConditionsAccessor = (*ClusterProvisionerStatus)(nil)

// ClusterProvisionerSpec is the spec for a ClusterProvisioner resource.
type ClusterProvisionerSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Reconciles is the kind of the resource the provisioner controller watches to
	// produce required  backing resources.
	// +required
	Reconciles metav1.GroupKind `json:"reconciles"`
}

var cProvCondSet = duck.NewLivingConditionSet()

// ClusterProvisionerStatus is the status for a ClusterProvisioner resource
type ClusterProvisionerStatus struct {
	// Conditions holds the state of a cluster provisioner at a point in time.
	Conditions duck.Conditions `json:"conditions,omitempty"`

	// ObservedGeneration is the 'Generation' of the ClusterProvisioner that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// GetSpecJSON returns spec as json
func (p *ClusterProvisioner) GetSpecJSON() ([]byte, error) {
	return json.Marshal(p.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterProvisionerList is a list of ClusterProvisioner resources
type ClusterProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ClusterProvisioner `json:"items"`
}

// GetConditions returns the Conditions array. This enables generic handling of
// conditions by implementing the duck.Conditions interface.
func (ps *ClusterProvisionerStatus) GetConditions() duck.Conditions {
	return ps.Conditions
}

// SetConditions sets the Conditions array. This enables generic handling of
// conditions by implementing the duck.Conditions interface.
func (ps *ClusterProvisionerStatus) SetConditions(conditions duck.Conditions) {
	ps.Conditions = conditions
}
