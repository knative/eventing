/*
Copyright 2020 The Knative Authors

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
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTypeDefinition represents a classification
type EventTypeDefinition struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EventTypeDefinition.
	Spec EventTypeDefinitionSpec `json:"spec,omitempty"`

	//	Status EventTypeDefinitionStatus `json:"status,omitempty"`
}

var (
	// Check that EventType can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*EventTypeDefinition)(nil)
	_ apis.Defaultable = (*EventTypeDefinition)(nil)

	// Check that EventType can return its spec untyped.
	_ apis.HasSpec = (*EventTypeDefinition)(nil)

	_ runtime.Object = (*EventTypeDefinition)(nil)

	// Check that we can create OwnerReferences to an EventType.
	_ kmeta.OwnerRefable = (*EventTypeDefinition)(nil)

	//// Check that the type conforms to the duck Knative Resource shape.
	//_ duckv1.KRShaped = (*EventTypeDefinition)(nil)
)

type EventTypeDefinitionSpec struct {
	// SchemaURL is a URI, it represents the payload schemaurl.
	// It may be a JSON schema, a protobuf schema, etc. It is optional.
	// +optional
	SchemaURL *apis.URL `json:"schemaUrl,omitempty"`

	// Describes the format of the events, but for us it is generally only CE...?
	Format string `json:"format,omitempty"`

	// The group where the EventTypeDefinition is defined.
	Group string `json:"group,omitempty"`

	// Event metadata, such as attributes, extensions from CloudEvents spec.
	Metadata EventTypeDefinitionMetadata `json:"metadata,inline"`

	// Description is an optional field used to describe the EventType, in any meaningful way.
	// +optional
	Description string `json:"description,omitempty"`
}

type EventTypeDefinitionMetadata struct {
	Attributes []EventTypeDefinitionAttribute `json:"attributes"`
}

type EventTypeDefinitionAttribute struct {
	Name     string `json:"name,omitempty"`
	Required bool   `json:"required"`
	Value    string `json:"value,omitempty"` //interface{}...
}

// EventTypeDefinitionStatus represents the current state of a EventType.
type EventTypeDefinitionStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTypeDefinitionList is a collection of EventTypes.
type EventTypeDefinitionList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventTypeDefinition `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventType
func (etd *EventTypeDefinition) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("EventTypeDefinition")
}

// GetUntypedSpec returns the spec of the EventType.
func (etd *EventTypeDefinition) GetUntypedSpec() interface{} {
	return etd.Spec
}

//// GetStatus retrieves the status of the EventType. Implements the KRShaped interface.
//func (t *EventTypeDefinition) GetStatus() *duckv1.Status {
//	return &t.Status.Status
//}
