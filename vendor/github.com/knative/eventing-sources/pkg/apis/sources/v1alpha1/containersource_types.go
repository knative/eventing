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

// Check that ContainerSource can be validated and can be defaulted.
var _ runtime.Object = (*ContainerSource)(nil)

// Check that ContainerSource implements the Conditions duck type.
var _ = duck.VerifyType(&ContainerSource{}, &duckv1alpha1.Conditions{})

// ContainerSourceSpec defines the desired state of ContainerSource
type ContainerSourceSpec struct {
	// Image is the image to run inside of the container.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image,omitempty"`

	// Args are passed to the ContainerSpec as they are.
	Args []string `json:"args,omitempty"`

	// Env is the list of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this
	// source.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

const (
	// ContainerSourceConditionReady has status True when the ContainerSource is ready to send events.
	ContainerConditionReady = duckv1alpha1.ConditionReady

	// ContainerConditionSinkProvided has status True when the ContainerSource has been configured with a sink target.
	ContainerConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// ContainerConditionDeployed has status True when the ContainerSource has had it's deployment created.
	ContainerConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var containerCondSet = duckv1alpha1.NewLivingConditionSet(
	ContainerConditionSinkProvided,
	ContainerConditionDeployed)

// ContainerSourceStatus defines the observed state of ContainerSource
type ContainerSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the ContainerSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *ContainerSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return containerCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *ContainerSourceStatus) IsReady() bool {
	return containerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *ContainerSourceStatus) InitializeConditions() {
	containerCondSet.Manage(s).InitializeConditions()
}

// MarSink sets the condition that the source has a sink configured.
func (s *ContainerSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		containerCondSet.Manage(s).MarkTrue(ContainerConditionSinkProvided)
	} else {
		containerCondSet.Manage(s).MarkUnknown(ContainerConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *ContainerSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkFalse(ContainerConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *ContainerSourceStatus) MarkDeployed() {
	containerCondSet.Manage(s).MarkTrue(ContainerConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *ContainerSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkUnknown(ContainerConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *ContainerSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkFalse(ContainerConditionDeployed, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSource is the Schema for the containersources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type ContainerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerSourceSpec   `json:"spec,omitempty"`
	Status ContainerSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSourceList contains a list of ContainerSource
type ContainerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerSource{}, &ContainerSourceList{})
}
