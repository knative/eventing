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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	// DependencyAnnotation is the annotation key used to mark the sources that the Trigger depends on.
	// This will be used when the kn client creates a source and trigger pair for the user such that the trigger only receives events produced by the paired source.
	DependencyAnnotation = "knative.dev/dependency"

	// InjectionAnnotation is the annotation key used to enable knative eventing
	// injection for a namespace to automatically create a broker.
	InjectionAnnotation = "eventing.knative.dev/injection"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Trigger represents a request to have events delivered to a subscriber from a
// Broker's event pool.
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

var (
	// Check that Trigger can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*Trigger)(nil)
	_ apis.Defaultable = (*Trigger)(nil)

	// Check that Trigger can return its spec untyped.
	_ apis.HasSpec = (*Trigger)(nil)

	_ runtime.Object = (*Trigger)(nil)

	// Check that we can create OwnerReferences to a Trigger.
	_ kmeta.OwnerRefable = (*Trigger)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*Trigger)(nil)
)

type TriggerSpec struct {
	// Broker is the broker that this trigger receives events from.
	Broker string `json:"broker,omitempty"`

	// Filter is the filter to apply against all events from the Broker. Only events that pass this
	// filter will be sent to the Subscriber. If not specified, will default to allowing all events.
	//
	// +optional
	Filter *TriggerFilter `json:"filter,omitempty"`

	// Filters is an experimental field that conforms to the CNCF CloudEvents Subscriptions
	// API. It's an array of filter expressions that evaluate to true or false.
	// If any filter expression in the array evaluates to false, the event MUST
	// NOT be sent to the Subscriber. If all the filter expressions in the array
	// evaluate to true, the event MUST be attempted to be delivered. Absence of
	// a filter or empty array implies a value of true. In the event of users
	// specifying both Filter and Filters, then the latter will override the former.
	// This will allow users to try out the effect of the new Filters field
	// without compromising the existing attribute-based Filter and try it out on existing
	// Trigger objects.
	//
	// +optional
	Filters []SubscriptionsAPIFilter `json:"filters,omitempty"`

	// Subscriber is the addressable that receives events from the Broker that pass
	// the Filter. It is required.
	Subscriber duckv1.Destination `json:"subscriber"`

	// Delivery contains the delivery spec for this specific trigger.
	// +optional
	Delivery *eventingduckv1.DeliverySpec `json:"delivery,omitempty"`
}

type TriggerFilter struct {
	// Attributes filters events by exact match on event context attributes.
	// Each key in the map is compared with the equivalent key in the event
	// context. An event passes the filter if all values are equal to the
	// specified values. Nested context attributes are not supported as keys. Only
	// string values are supported.
	//
	// +optional
	Attributes TriggerFilterAttributes `json:"attributes,omitempty"`
}

// SubscriptionsAPIFilter allows defining a filter expression using CloudEvents
// Subscriptions API. If multiple filters are specified, then the same semantics
// of SubscriptionsAPIFilter.All is applied. If no filter dialect or empty
// object is specified, then the filter always accept the events.
type SubscriptionsAPIFilter struct {
	// All evaluates to true if all the nested expressions evaluate to true.
	// It must contain at least one filter expression.
	//
	// +optional
	All []SubscriptionsAPIFilter `json:"all,omitempty"`

	// Any evaluates to true if at least one of the nested expressions evaluates
	// to true. It must contain at least one filter expression.
	//
	// +optional
	Any []SubscriptionsAPIFilter `json:"any,omitempty"`

	// Not evaluates to true if the nested expression evaluates to false.
	//
	// +optional
	Not *SubscriptionsAPIFilter `json:"not,omitempty"`

	// Exact evaluates to true if the value of the matching CloudEvents
	// attribute matches exactly the String value specified (case-sensitive).
	// Exact must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Exact map[string]string `json:"exact,omitempty"`

	// Prefix evaluates to true if the value of the matching CloudEvents
	// attribute starts with the String value specified (case-sensitive). Prefix
	// must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Prefix map[string]string `json:"prefix,omitempty"`

	// Suffix evaluates to true if the value of the matching CloudEvents
	// attribute ends with the String value specified (case-sensitive). Suffix
	// must contain exactly one property, where the key is the name of the
	// CloudEvents attribute to be matched, and its value is the String value to
	// use in the comparison. The attribute name and value specified in the filter
	// expression cannot be empty strings.
	//
	// +optional
	Suffix map[string]string `json:"suffix,omitempty"`

	// SQL is a CloudEvents SQL expression that will be evaluated to true or false against each CloudEvent.
	//
	// +optional
	SQL string `json:"sql,omitempty"`
}

// TriggerFilterAttributes is a map of context attribute names to values for
// filtering by equality. Only exact matches will pass the filter. You can use
// the value '' to indicate all strings match.
type TriggerFilterAttributes map[string]string

// TriggerStatus represents the current state of a Trigger.
type TriggerStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Trigger that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// SubscriberURI is the resolved URI of the receiver for this Trigger.
	// +optional
	SubscriberURI *apis.URL `json:"subscriberUri,omitempty"`

	// DeliveryStatus contains a resolved URL to the dead letter sink address, and any other
	// resolved delivery options.
	eventingduckv1.DeliveryStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TriggerList is a collection of Triggers.
type TriggerList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Trigger `json:"items"`
}

// GetStatus retrieves the status of the Trigger. Implements the KRShaped interface.
func (t *Trigger) GetStatus() *duckv1.Status {
	return &t.Status.Status
}
