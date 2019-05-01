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

type TriggerSpec struct {
	// Broker is the broker that this trigger receives events from. If not specified, will default
	// to 'default'.
	Broker string `json:"broker,omitempty"`

	// Filter is the filter to apply against all events from the Broker. Only events that pass this
	// filter will be sent to the Subscriber. If not specified, will default to allowing all events.
	//
	// +optional
	Filter *TriggerFilter `json:"filter,omitempty"`

	// Subscriber is the addressable that receives events from the Broker that pass the Filter. It
	// is required.
	Subscriber *SubscriberSpec `json:"subscriber,omitempty"`
}

// TriggerFilter specifies the event filtering strategy for the Trigger. Only
// one field may be set.
type TriggerFilter struct {
	// DeprecatedSourceAndType filters events based on exact matches on the
	// CloudEvents type and source attributes. This field has been replaced by the
	// Attributes field.
	//
	// +optional
	DeprecatedSourceAndType *TriggerFilterSourceAndType `json:"sourceAndType,omitempty"`

	// Attributes filters events by exact match on event context attributes.
	// Each key in the map is compared with the equivalent key in the event
	// context. An event passes the filter if all values are equal to the
	// specified values.
	//
	// Nested context attributes are not supported as keys. Numeric values are
	// not supported.
	Attributes *TriggerFilterAttributes `json:"attributes,omitempty`

	// Expression filters events by evaluating the expression with the Common
	// Expression Language runtime. An event passes the filter if the expression
	// evaluates to true.
	//
	// +optional
	Expression *TriggerFilterExpression `json:"expression,omitempty"`
}

// TriggerFilterSourceAndType filters events based on exact matches on the cloud event's type and
// source attributes. Only exact matches will pass the filter. Either or both type and source can
// use the value 'Any' to indicate all strings match.
type TriggerFilterSourceAndType struct {
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
}

// TriggerFilterAttributes is a map of context attribute names to values for
// filtering by equality.
type TriggerFilterAttributes map[string]string

// TriggerFilterExpression is a string containing the filter expression to
// evaluate.
type TriggerFilterExpression string

// TriggerStatus represents the current state of a Trigger.
type TriggerStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SubscriberURI is the resolved URI of the receiver for this Trigger.
	SubscriberURI string `json:"subscriberURI,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TriggerList is a collection of Triggers.
type TriggerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trigger `json:"items"`
}
