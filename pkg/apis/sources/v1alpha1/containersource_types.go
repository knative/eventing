/*
Copyright 2019 The Knative Authors

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
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// ContainerSource is the Schema for the containersources API
type ContainerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerSourceSpec   `json:"spec,omitempty"`
	Status ContainerSourceStatus `json:"status,omitempty"`
}

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

// GetGroupVersionKind returns the GroupVersionKind.
func (s *ContainerSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ContainerSource")
}

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

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSourceList contains a list of ContainerSource
type ContainerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerSource `json:"items"`
}
