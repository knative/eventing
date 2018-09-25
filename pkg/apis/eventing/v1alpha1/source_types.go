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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Source resource Describes a specific configuration (credentials, etc) of a
// source system which can be used to supply events. Sources emit events using a
// channel specified in their status. They cannot receive events.
type Source struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the the Provisioner and arguments provided for this Source.
	Spec SourceSpec `json:"spec"`

	// Status is the current status of the Source.
	// +optional
	Status SourceStatus `json:"status,omitempty"`
}

// Check that Source can be validated and can be defaulted.
var _ webhook.GenericCRD = (*Source)(nil)

// Check that Source implements the Conditions duck type.
var _ = duck.VerifyType(&Source{}, &duckv1alpha1.Conditions{})

// Check that Source implements the Generation duck type.
var emptyGenSource duckv1alpha1.Generation
var _ = duck.VerifyType(&Source{}, &emptyGenSource)

// And it's Subscribable
var _ = duck.VerifyType(&Subscription{}, &duckv1alpha1.Subscribable{})

// SourceSpec is the spec for a Source resource.
type SourceSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Provisioner is used to create any backing resources and configuration.
	// +required
	Provisioner *ProvisionerReference `json:"provisioner,omitempty"`

	// Arguments defines the arguments to pass to the Provisioner which provisions
	// this Source.
	// +optional
	Arguments *runtime.RawExtension `json:"arguments,omitempty"`

	// Specify an existing channel to use to emit events. If empty, create a new
	// Channel using the cluster/namespace default.
	//
	// This object must fulfill the Channelable contract.
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// Currently Kind must be "Channel" and
	// APIVersion must be "eventing.knative.dev/v1alpha1"
	// +optional
	Channel *corev1.ObjectReference `json:"target,omitempty"`
}

const (
	// SourceConditionReady has status True when the Source is ready to send events.
	SourceConditionReady = duckv1alpha1.ConditionReady

	// SourceConditionProvisioned has status True when the Source's backing
	// resources have been provisioned.
	SourceConditionProvisioned duckv1alpha1.ConditionType = "Provisioned"
)

var sourceCondSet = duckv1alpha1.NewLivingConditionSet(SourceConditionProvisioned)

// SourceStatus is the status for a Source resource
type SourceStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the 'Generation' of the Source that
	// was last reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Source might be Subscribable. This points to the Channelable object.
	Subscribable duckv1alpha1.Subscribable `json:"subscribable,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (ss *SourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return sourceCondSet.Manage(ss).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ss *SourceStatus) IsReady() bool {
	return sourceCondSet.Manage(ss).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ss *SourceStatus) InitializeConditions() {
	sourceCondSet.Manage(ss).InitializeConditions()
}

// MarkProvisioned sets the condition that the source has had its backing resources created.
func (ss *SourceStatus) MarkProvisioned() {
	sourceCondSet.Manage(ss).MarkTrue(SourceConditionProvisioned)
}

// MarkDeprovisioned sets the condition that the source has had its backing resources removed.
func (ss *SourceStatus) MarkDeprovisioned(reason, messageFormat string, messageA ...interface{}) {
	sourceCondSet.Manage(ss).MarkFalse(SourceConditionProvisioned, reason, messageFormat, messageA)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SourceList is a list of Source resources
type SourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Source `json:"items"`
}
