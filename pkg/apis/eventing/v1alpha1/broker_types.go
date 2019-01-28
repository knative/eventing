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

// Broker is an abstract resource that implements the Addressable contract.
// The Provisioner provisions infrastructure to accepts events and
// deliver to Subscriptions.
type Broker struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Broker.
	Spec BrokerSpec `json:"spec,omitempty"`

	// Status represents the current state of the Broker. This data may be out of
	// date.
	// +optional
	Status BrokerStatus `json:"status,omitempty"`
}

// Check that Broker can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Broker)(nil)
var _ apis.Defaultable = (*Broker)(nil)
var _ apis.Immutable = (*Broker)(nil)
var _ runtime.Object = (*Broker)(nil)
var _ webhook.GenericCRD = (*Broker)(nil)

// BrokerSpec specifies the Provisioner backing a channel and the configuration
// arguments for a Broker.
type BrokerSpec struct {
	Selector              *metav1.LabelSelector     `json:"selector,omitempty"`
	ChannelTemplate       *ChannelTemplateSpec      `json:"channelTemplate,omitempty"`
	SubscribableResources []metav1.GroupVersionKind `json:"subscribableResources,omitempty"`
}

type ChannelTemplateSpec struct {
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec     *ChannelSpec      `json:"spec,omitempty"`
}

var brokerCondSet = duckv1alpha1.NewLivingConditionSet(BrokerConditionChannelTemplateSelector, BrokerConditionSubscribableResourcesExist)

// BrokerStatus represents the current state of a Broker.
type BrokerStatus struct {
	// ObservedGeneration is the most recent generation observed for this Broker.
	// It corresponds to the Broker's generation, which is updated on mutation by
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
	// BrokerConditionReady has status True when the Broker is ready to
	// accept traffic.
	BrokerConditionReady = duckv1alpha1.ConditionReady

	BrokerConditionChannelTemplateSelector duckv1alpha1.ConditionType = "ChannelTemplateSelector"

	BrokerConditionSubscribableResourcesExist duckv1alpha1.ConditionType = "SubscribableResourcesExist"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (bs *BrokerStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return brokerCondSet.Manage(bs).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (bs *BrokerStatus) IsReady() bool {
	return brokerCondSet.Manage(bs).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (bs *BrokerStatus) InitializeConditions() {
	brokerCondSet.Manage(bs).InitializeConditions()
}

func (bs *BrokerStatus) MarkChannelTemplateMatchesSelector() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionChannelTemplateSelector)
}

func (bs *BrokerStatus) MarkChannelTemplateDoesNotMatchSelector() {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionChannelTemplateSelector, "selectorDoesNotMatchTemplate", "`spec.selector` does not match `spec.channelTempalte.meta.labels`")
}

// MarkProvisioned sets BrokerConditionProvisioned condition to True state.
func (bs *BrokerStatus) MarkSubscribableResourcesExist() {
	brokerCondSet.Manage(bs).MarkTrue(BrokerConditionSubscribableResourcesExist)
}

// MarkNotProvisioned sets BrokerConditionProvisioned condition to False state.
func (bs *BrokerStatus) MarkSubscribableResourcesDoNotExist(reason, messageFormat string, messageA ...interface{}) {
	brokerCondSet.Manage(bs).MarkFalse(BrokerConditionSubscribableResourcesExist, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BrokerList is a collection of Brokers.
type BrokerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Broker `json:"items"`
}
