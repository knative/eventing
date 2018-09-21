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

	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Source
type Source struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the Types provisioned by this Source.
	Spec SourceSpec `json:"spec"`

	// Status is the current status of the Source.
	// +optional
	Status SourceStatus `json:"status,omitempty"`
}

// Check that Source can be validated and can be defaulted.
var _ webhook.GenericCRD = (*Source)(nil)

// Check that SourceStatus may have its conditions managed.
var _ duckv1alpha1.ConditionsAccessor = (*SourceStatus)(nil)

// Check that Source implements the Conditions duck type.
var _ = duck.VerifyType(&Source{}, &duckv1alpha1.Conditions{})

// Check that Source implements the Generation duck type.
var emptyGen duckv1alpha1.Generation
var _ = duck.VerifyType(&Source{}, &emptyGen)

// SourceSpec is the spec for a Source resource.
type SourceSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	Todo string `json:"todo,omitempty"`
	// TODO: add real params.
}

var sourceCondSet = duckv1alpha1.NewLivingConditionSet()

// SourceStatus is the status for a Source resource
type SourceStatus struct {
	// Conditions holds the state of a cluster provisioner at a point in time.
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the 'Generation' of the Source that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (ss *SourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return sourceCondSet.Manage(ss).GetCondition(t)
}

// GetConditions returns the Conditions array. This enables generic handling of
// conditions by implementing the duckv1alpha1.Conditions interface.
func (ss *SourceStatus) GetConditions() duckv1alpha1.Conditions {
	return ss.Conditions
}

// IsReady returns true if the resource is ready overall.
func (ss *SourceStatus) IsReady() bool {
	return sourceCondSet.Manage(ss).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ss *SourceStatus) InitializeConditions() {
	sourceCondSet.Manage(ss).InitializeConditions()
}

// SetConditions sets the Conditions array. This enables generic handling of
// conditions by implementing the duckv1alpha1.Conditions interface.
func (ss *SourceStatus) SetConditions(conditions duckv1alpha1.Conditions) {
	ss.Conditions = conditions
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceList is a list of Source resources
type SourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Source `json:"items"`
}
