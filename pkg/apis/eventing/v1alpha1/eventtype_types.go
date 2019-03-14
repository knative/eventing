/*
 * Copyright 2019 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type EventType struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EventType.
	Spec EventTypeSpec `json:"spec,omitempty"`

	// Status represents the current state of the EventType.
	// This data may be out of date.
	// +optional
	Status EventTypeStatus `json:"status,omitempty"`
}

// Check that EventType can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*EventType)(nil)
var _ apis.Defaultable = (*EventType)(nil)
var _ apis.Immutable = (*EventType)(nil)
var _ runtime.Object = (*EventType)(nil)
var _ webhook.GenericCRD = (*EventType)(nil)

type EventTypeSpec struct {
	// TODO By enabling the status subresource metadata.generation should increment
	// thus making this property obsolete.
	//
	// We should be able to drop this property with a CRD conversion webhook
	// in the future
	//
	// +optional
	DeprecatedGeneration int64 `json:"generation,omitempty"`

	// TODO these attributes should be updated once we clarify the UX.
	// Type refers to the cloud event type.
	Type string `json:"type,omitempty"`
	// +optional
	From string `json:"from,omitempty"`
	// +optional
	Schema string `json:"schema,omitempty"`
}

var eventTypeCondSet = duckv1alpha1.NewLivingConditionSet(EventTypeConditionReady)

// EventTypeStatus represents the current state of a EventType.
type EventTypeStatus struct {
	// ObservedGeneration is the most recent generation observed for this EventType.
	// It corresponds to the Broker's generation, which is updated on mutation by
	// the API Server.
	// TODO: The above comment is only true once
	// https://github.com/kubernetes/kubernetes/issues/58778 is fixed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of a event type's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

const (
	EventTypeConditionReady = duckv1alpha1.ConditionReady
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (et *EventTypeStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return eventTypeCondSet.Manage(et).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (et *EventTypeStatus) IsReady() bool {
	return eventTypeCondSet.Manage(et).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (et *EventTypeStatus) InitializeConditions() {
	eventTypeCondSet.Manage(et).InitializeConditions()
}

func (rs *EventTypeStatus) MarkEventTypeReady() {
	eventTypeCondSet.Manage(rs).MarkTrue(EventTypeConditionReady)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventTypeList is a collection of EventTypes.
type EventTypeList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventType `json:"items"`
}
