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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigMapPropagation is used to propagate configMaps from source namespace to target namespace
type ConfigMapPropagation struct {
	metav1.TypeMeta `json:". inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the ConfigMapPropagation
	Spec ConfigMapPropagationSpec `json:"spec,omitempty"`

	// Status represents the current state of the EventType.
	// This data may be out of date.
	// +optional
	Status ConfigMapPropagationStatus `json:"status,omitempty"`
}

var (
	// Check that ConfigMapPropagation can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*ConfigMapPropagation)(nil)
	_ apis.Defaultable = (*ConfigMapPropagation)(nil)

	// Check that ConfigMapPropagation can return its spec untyped.
	_ apis.HasSpec = (*ConfigMapPropagation)(nil)

	_ runtime.Object = (*ConfigMapPropagation)(nil)

	// Check that we can create OwnerReferences to an ConfigMapPropagation.
	_ kmeta.OwnerRefable = (*ConfigMapPropagation)(nil)
)

type ConfigMapPropagationSpec struct {
	// OriginalNamespace represents the namespace where the original configMaps are in
	OriginalNamespace string `json:"originalNamespace,omitempty"`
	// Selector only selects original configMaps with labels under Selector
	Selector string `json:"selector,omitempty"`
}

// ConfigMapPropagationStatus represents the current state of a ConfigMapPropagation.
type ConfigMapPropagationStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigMapPropagationList is a collection of EventTypes.
type ConfigMapPropagationList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigMapPropagation `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventType
func (cmp *ConfigMapPropagation) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("ConfigMapPropagation")
}

// GetUntypedSpec returns the spec of the EventType.
func (cmp *ConfigMapPropagation) GetUntypedSpec() interface{} {
	return cmp.Spec
}
