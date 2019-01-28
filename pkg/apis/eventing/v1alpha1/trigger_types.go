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

// Trigger is an abstract resource that implements the Addressable contract.
// The Provisioner provisions infrastructure to accepts events and
// deliver to Subscriptions.
type Trigger struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Trigger.
	Spec TriggerSpec `json:"spec,omitempty"`

	// Status represents the current state of the Trigger. This data may be out of
	// date.
	// +optional
	Status TriggerStatus `json:"status,omitempty"`
}

// Check that Trigger can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Trigger)(nil)
var _ apis.Defaultable = (*Trigger)(nil)
var _ apis.Immutable = (*Trigger)(nil)
var _ runtime.Object = (*Trigger)(nil)
var _ webhook.GenericCRD = (*Trigger)(nil)

// TriggerSpec specifies the Provisioner backing a channel and the configuration
// arguments for a Trigger.
type TriggerSpec struct {
	Broker     string               `json:"broker,omitempty"`
	Selector   *TriggerSelectorSpec `json:"selector,omitempty"`
	Subscriber *SubscriberSpec      `json:"subscriber,omitempty"`
}

type TriggerSelectorSpec struct {
	Header           map[string]string                 `json:"header,omitempty"`
	HeaderExpression []metav1.LabelSelectorRequirement `json:"headerExpression,omitEmpty"`
	OPAPolicy        string                            `json:"opaPolicy,omitEmpty"`
}

var triggerCondSet = duckv1alpha1.NewLivingConditionSet(TriggerConditionBrokerExists, TriggerConditionSubscriberFound)

// TriggerStatus represents the current state of a Trigger.
type TriggerStatus struct {
	// ObservedGeneration is the most recent generation observed for this Trigger.
	// It corresponds to the Trigger's generation, which is updated on mutation by
	// the API Server.
	// TODO: The above comment is only true once
	// https://github.com/kubernetes/kubernetes/issues/58778 is fixed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of a channel's current state.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

const (
	// TriggerConditionReady has status True when the Trigger is ready to
	// accept traffic.
	TriggerConditionReady = duckv1alpha1.ConditionReady

	TriggerConditionBrokerExists duckv1alpha1.ConditionType = "BrokerExists"

	TriggerConditionSubscriberFound duckv1alpha1.ConditionType = "SubscriberFound"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ts *TriggerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return triggerCondSet.Manage(ts).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ts *TriggerStatus) IsReady() bool {
	return triggerCondSet.Manage(ts).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ts *TriggerStatus) InitializeConditions() {
	triggerCondSet.Manage(ts).InitializeConditions()
}

func (ts *TriggerStatus) MarkBrokerExists() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionBrokerExists)
}

func (ts *TriggerStatus) MarkBrokerDoesNotExists() {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionBrokerExists, "doesNotExist", "Broker does not exist")
}

// MarkProvisioned sets TriggerConditionProvisioned condition to True state.
func (ts *TriggerStatus) MarkSubscriberFound() {
	triggerCondSet.Manage(ts).MarkTrue(TriggerConditionSubscriberFound)
}

// MarkNotProvisioned sets TriggerConditionProvisioned condition to False state.
func (ts *TriggerStatus) MarkSubscriberNotFound(reason, messageFormat string, messageA ...interface{}) {
	triggerCondSet.Manage(ts).MarkFalse(TriggerConditionSubscriberFound, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TriggerList is a collection of Triggers.
type TriggerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trigger `json:"items"`
}
