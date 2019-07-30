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
	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/webhook"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// Choice defines conditional branches that will be wired in
// series through Channels and Subscriptions.
type Choice struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Choice.
	Spec ChoiceSpec `json:"spec,omitempty"`

	// Status represents the current state of the Choice. This data may be out of
	// date.
	// +optional
	Status ChoiceStatus `json:"status,omitempty"`
}

// Check that Choice can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Choice)(nil)
var _ apis.Defaultable = (*Choice)(nil)

// TODO: make appropriate fields immutable.
//var _ apis.Immutable = (*Choice)(nil)
var _ runtime.Object = (*Choice)(nil)
var _ webhook.GenericCRD = (*Choice)(nil)

type ChoiceSpec struct {
	// Cases is the list of Filter/Subscribers pairs. Filters are evaluated in the order
	// provided, until one pass (returns true)
	Cases []ChoiceCase `json:"cases"`

	// ChannelTemplate specifies which Channel CRD to use. If left unspecified, it is set to the default Channel CRD
	// for the namespace (or cluster, in case there are no defaults for the namespace).
	// +optional
	ChannelTemplate *eventingduckv1alpha1.ChannelTemplateSpec `json:"channelTemplate"`

	// Reply is a Reference to where the result of a case Subscriber gets sent to
	// when the case does not have a Reply
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	//
	//  The resource pointed by this ObjectReference must meet the Addressable contract
	//  with a reference to the Addressable duck type. If the resource does not meet this contract,
	//  it will be reflected in the Subscription's status.
	// +optional
	Reply *corev1.ObjectReference `json:"reply,omitempty"`
}

type ChoiceCase struct {
	// Filter is the expression guarding the branch/case
	Filter *eventingv1alpha1.SubscriberSpec `json:"filter,omitempty"`

	// Subscriber receiving the event when the filter passes
	Subscriber eventingv1alpha1.SubscriberSpec `json:"subscriber"`

	// Reply is a Reference to where the result of Subscriber of this case gets sent to.
	// If not specified, sent the result to the Choice Reply
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	//
	//  The resource pointed by this ObjectReference must meet the Addressable contract
	//  with a reference to the Addressable duck type. If the resource does not meet this contract,
	//  it will be reflected in the Subscription's status.
	// +optional
	Reply *corev1.ObjectReference `json:"reply,omitempty"`
}

// ChoiceStatus represents the current state of a Choice.
type ChoiceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// IngressChannelStatus corresponds to the ingress channel status.
	IngressChannelStatus ChoiceChannelStatus `json:"ingressChannelStatus"`

	// CaseStatuses is an array of corresponding to cases status.
	// Matches the Spec.Cases array in the order.
	CaseStatuses []ChoiceCaseStatus `json:"caseStatuses"`

	// AddressStatus is the starting point to this Choice. Sending to this
	// will target the first subscriber.
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1alpha1.AddressStatus `json:",inline"`
}

// ChoiceCaseStatus represents the current state of a Choice case
type ChoiceCaseStatus struct {
	// FilterSubscriptionStatus corresponds to the filter subscription status.
	FilterSubscriptionStatus ChoiceSubscriptionStatus `json:"filterSubscriptionStatus"`

	// FilterChannelStatus corresponds to the filter channel status.
	FilterChannelStatus ChoiceChannelStatus `json:"filterChannelStatus"`

	// SubscriptionStatus corresponds to the subscriber subscription status.
	SubscriptionStatus ChoiceSubscriptionStatus `json:"subscriberSubscriptionStatus"`
}

type ChoiceChannelStatus struct {
	// Channel is the reference to the underlying channel.
	Channel corev1.ObjectReference `json:"channel"`

	// ReadyCondition indicates whether the Channel is ready or not.
	ReadyCondition apis.Condition `json:"ready"`
}

type ChoiceSubscriptionStatus struct {
	// Subscription is the reference to the underlying Subscription.
	Subscription corev1.ObjectReference `json:"subscription"`

	// ReadyCondition indicates whether the Subscription is ready or not.
	ReadyCondition apis.Condition `json:"ready"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChoiceList is a collection of Choices.
type ChoiceList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Choice `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for Choice
func (p *Choice) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Choice")
}
