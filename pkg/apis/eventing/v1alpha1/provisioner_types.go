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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/webhook"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Provisioner provisions.
type Provisioner struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProvisionerSpec   `json:"spec"`
	Status ProvisionerStatus `json:"status"`
}

// Check that Provisioner can be validated and can be defaulted.
var _ apis.Validatable = (*Provisioner)(nil)
var _ apis.Defaultable = (*Provisioner)(nil)
var _ runtime.Object = (*Provisioner)(nil)
var _ webhook.GenericCRD = (*Provisioner)(nil)

// ProvisionerSpec is the spec for a Provisioner resource.
type ProvisionerSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Type is the type of the resource to be provisioned.
	// +required
	Type runtime.TypeMeta `json:"type"`

	// Parameters are used for validation of arguments.
	// +optional
	Parameters []ParameterSpec `json:"parameters,omitempty"`

	// Service Account to use when creating the underlying objects.
	// defaults to "default"
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

type ProvisionerConditionStatus ConditionStatus

// ProvisionerStatus is the status for a Provisioner resource
type ProvisionerStatus struct {
	// Conditions holds the state of a provisioner at a point in time.
	Conditions []ProvisionerConditionStatus `json:"conditions,omitempty"`

	// Provisioned holds the status of creation or adoption of each EventType
	// and errors therein. It is expected that a provisioner list all produced
	// EventTypes, if applicable.
	Provisioned []ProvisionedStatus `json:"provisioned,omitempty"`

	// ProvisionerContext is what the Provisioner operation returns and holds
	// enough information to perform cleanup once a Provisioner is deleted.
	// NOTE: experimental field.
	ProvisionerContext *runtime.RawExtension `json:"provisionerContext,omitempty"`

	// ObservedGeneration is the 'Generation' of the Provisioner that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// GetSpecJSON returns spec as json
func (p *Provisioner) GetSpecJSON() ([]byte, error) {
	return json.Marshal(p.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ProvisionerList is a list of Provisioner resources
type ProvisionerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Provisioner `json:"items"`
}
