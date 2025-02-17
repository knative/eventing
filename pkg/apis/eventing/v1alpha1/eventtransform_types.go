/*
Copyright 2025 The Knative Authors

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
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTransform represents an even transformation.
type EventTransform struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EventTransform.
	Spec EventTransformSpec `json:"spec,omitempty"`

	// Status represents the current state of the EventTransform.
	// This data may be out of date.
	// +optional
	Status EventTransformStatus `json:"status,omitempty"`
}

var (
	// Check that EventTransform can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*EventTransform)(nil)
	_ apis.Defaultable = (*EventTransform)(nil)

	// Check that EventTransform can return its spec untyped.
	_ apis.HasSpec = (*EventTransform)(nil)

	_ runtime.Object = (*EventTransform)(nil)

	// Check that we can create OwnerReferences to an EventTransform.
	_ kmeta.OwnerRefable = (*EventTransform)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*EventTransform)(nil)
)

type EventTransformSpec struct {

	// Sink is a reference to an object that will resolve to a uri to use as the sink.
	//
	// If not present, the transformation will send back the transformed event as response, this
	// is useful to leverage the built-in Broker reply feature to re-publish a transformed event
	// back to the broker.
	//
	// +optional
	Sink *duckv1.Destination `json:"sink,omitempty"`

	// Reply is the configuration on how to handle responses from Sink.
	// It can only be set if Sink is set.
	//
	// +optional
	Reply *ReplySpec `json:"reply,omitempty"`

	// EventTransformations contain all possible transformations, only one "type" can be used.
	EventTransformations `json:",inline"`
}

// ReplySpec is the configurations on how to handle responses from Sink.
type ReplySpec struct {
	// EventTransformations for replies from the Sink, contain all possible transformations,
	// only one "type" can be used.
	//
	// The used type must match the top-level transformation, if you need to mix transformation types,
	// use compositions and chain transformations together to achieve your desired outcome.
	EventTransformations `json:",inline"`

	// Discard discards responses from Sink and return empty response body.
	//
	// When set to false, it returns the exact sink response body.
	// When set to true, Discard is mutually exclusive with EventTransformations in the reply
	// section, it can either be discarded or transformed.
	//
	// Default: false.
	//
	// +optional
	Discard *bool `json:"discard,omitempty"`
}

type EventTransformations struct {
	Jsonata *JsonataEventTransformationSpec `json:"jsonata,omitempty"`
}

type JsonataEventTransformationSpec struct {
	// Expression is the JSONata expression.
	Expression string `json:"expression,omitempty"`
}

// EventTransformStatus represents the current state of a EventTransform.
type EventTransformStatus struct {
	// SourceStatus inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`

	// AddressStatus is the part where the EventTransform fulfills the Addressable contract.
	// It exposes the endpoint as an URI to get events delivered.
	// +optional
	duckv1.AddressStatus `json:",inline"`

	// JsonataTransformationStatus is the status associated with JsonataEventTransformationSpec.
	// +optional
	JsonataTransformationStatus *JsonataEventTransformationStatus `json:"jsonata,omitempty"`
}

type JsonataEventTransformationStatus struct {
	Deployment appsv1.DeploymentStatus `json:"deployment,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTransformList is a collection of EventTransform.
type EventTransformList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventTransform `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventTransform
func (ep *EventTransform) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("EventTransform")
}

// GetUntypedSpec returns the spec of the EventTransform.
func (ep *EventTransform) GetUntypedSpec() interface{} {
	return ep.Spec
}

// GetStatus retrieves the status of the EventTransform. Implements the KRShaped interface.
func (ep *EventTransform) GetStatus() *duckv1.Status {
	return &ep.Status.Status
}
