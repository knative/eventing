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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Check that KubernetesEventSource can be validated and can be defaulted.
var _ runtime.Object = (*KubernetesEventSource)(nil)

// Check that KubernetesEventSource implements the Conditions duck type.
var _ = duck.VerifyType(&KubernetesEventSource{}, &duckv1alpha1.Conditions{})

// KubernetesEventSourceSpec defines the desired state of the source.
type KubernetesEventSourceSpec struct {
	// Namespace that we watch kubernetes events in.
	Namespace string `json:"namespace"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this
	// source.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use
	// as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

const (
	// KubernetesEventSourceConditionReady has status True when the
	// source is ready to send events.
	KubernetesEventSourceConditionReady = duckv1alpha1.ConditionReady
)

var kubernetesEventSourceCondSet = duckv1alpha1.NewLivingConditionSet()

// KubernetesEventSourceStatus defines the observed state of the source.
type KubernetesEventSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the source.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *KubernetesEventSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return kubernetesEventSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *KubernetesEventSourceStatus) IsReady() bool {
	return kubernetesEventSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *KubernetesEventSourceStatus) InitializeConditions() {
	kubernetesEventSourceCondSet.Manage(s).InitializeConditions()
}

// MarkReady sets the condition that the ContainerSource owned by
// the source has Ready status True.
func (s *KubernetesEventSourceStatus) MarkReady() {
	kubernetesEventSourceCondSet.Manage(s).MarkTrue(KubernetesEventSourceConditionReady)
}

// MarkUnready sets the condition that the ContainerSource owned by
// the source does not have Ready status True.
func (s *KubernetesEventSourceStatus) MarkUnready(reason, messageFormat string, messageA ...interface{}) {
	kubernetesEventSourceCondSet.Manage(s).MarkFalse(KubernetesEventSourceConditionReady, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesEventSource is the Schema for the kuberneteseventsources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type KubernetesEventSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubernetesEventSourceSpec   `json:"spec,omitempty"`
	Status KubernetesEventSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesEventSourceList contains a list of KubernetesEventSource
type KubernetesEventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubernetesEventSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubernetesEventSource{}, &KubernetesEventSourceList{})
}
