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
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RequestRepluy represents synchronous interface to sending and receiving events from a Broker.
type RequestReply struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EventPolicy.
	Spec RequestReplySpec `json:"spec,omitempty"`

	// Status represents the current state of the EventPolicy.
	// This data may be out of date.
	// +optional
	Status RequestReplyStatus `json:"status,omitempty"`
}

var (
	// Check that EventPolicy can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*RequestReply)(nil)
	_ apis.Defaultable = (*RequestReply)(nil)

	// Check that EventPolicy can return its spec untyped.
	_ apis.HasSpec = (*RequestReply)(nil)

	_ runtime.Object = (*RequestReply)(nil)

	// Check that we can create OwnerReferences to an EventPolicy.
	_ kmeta.OwnerRefable = (*RequestReply)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*RequestReply)(nil)
)

type RequestReplySpec struct {
	// BrokerRef contains the reference to the broker the RequestReply sends events to.
	BrokerRef duckv1.KReference `json:"brokerRef"`

	CorrelationAttribute string `json:"correlationAttribute"`

	ReplyAttribute string `json:"replyAttribute"`

	Timeout *string `json:"timeout,omitempty"`

	Delivery *eventingduckv1.DeliverySpec `json:"delivery,omitempty"`
}

// RequestReplyStatus represents the current state of a RequestReply.
type RequestReplyStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// AddressStatus is the part where the RequestReply fulfills the Addressable contract.
	// It exposes the endpoint as an URI to get events delivered.
	// +optional
	duckv1.AddressStatus `json:",inline"`

	// AppliedEventPoliciesStatus contains the list of EventPolicies which apply to this Broker.
	// +optional
	eventingduckv1.AppliedEventPoliciesStatus `json:",inline"`

	// DesiredReplicas is the number of replicas (StatefulSet pod + trigger) that is desired
	DesiredReplicas *int32 `json:"desiredReplicas,omitempty"`

	// ReadyReplicas is the number of ready replicas (StatefulSet pod + trigger) for this RequestReply resource
	ReadyReplicas *int32 `json:"readyReplicas,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RequestReplyList is a collection of RequestReplies.
type RequestReplyList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RequestReply `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventPolicy
func (rr *RequestReply) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("RequestReply")
}

// GetUntypedSpec returns the spec of the EventPolicy.
func (rr *RequestReply) GetUntypedSpec() interface{} {
	return rr.Spec
}

// GetStatus retrieves the status of the EventPolicy. Implements the KRShaped interface.
func (rr *RequestReply) GetStatus() *duckv1.Status {
	return &rr.Status.Status
}

func (rr *RequestReply) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Name:      rr.Name,
		Namespace: rr.Namespace,
	}
}
