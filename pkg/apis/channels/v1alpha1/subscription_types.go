/*
 * Copyright 2018 The Knative Authors
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
	"encoding/json"

	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/webhook"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Subscription routes events received on a Channel to a DNS name and
// corresponds to the subscriptions.channels.knative.dev CRD.
type Subscription struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               SubscriptionSpec   `json:"spec"`
	Status             SubscriptionStatus `json:"status,omitempty"`
}

// Check that Subscription can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Subscription)(nil)
var _ apis.Defaultable = (*Subscription)(nil)
var _ apis.Immutable = (*Subscription)(nil)
var _ runtime.Object = (*Subscription)(nil)
var _ webhook.GenericCRD = (*ClusterBus)(nil)

// SubscriptionSpec specifies the Channel for incoming events, a handler and the Channel
// for outgoing messages.
// from --[transform]--> to
// Note that the following are valid configurations also:
// Sink, no outgoing events:
// from -- transform
// no-op function (identity transformation):
// from --> to
type SubscriptionSpec struct {
	// TODO: Generation does not work correctly with CRD. They are scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once that gets fixed, remove this and use
	// ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Reference to an object that will be used to create the subscription
	// for receiving events. The object must have spec.subscriptions
	// list which will then be modified accordingly.
	//
	// This object must fulfill the Subscribable contract.
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	From *corev1.ObjectReference `json:"from,omitempty"`

	// Processor is reference to (optional) function for processing events.
	// Events from the From channel will be delivered here and replies
	// are sent to To channel.
	//
	// This object must fulfill the Targetable contract.
	//
	// Reference to an object that will be used to deliver events for
	// (optional) processing before sending them to To for further
	// if specified for additional Subscriptions to then subscribe
	// to these events for further processing.
	//
	// For example, this could be a reference to a Route resource
	// or a Service resource.
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	Processor *corev1.ObjectReference `json:"processor,omitempty"`

	// To is the (optional) resolved channel where (optionally) processed
	// events get sent.
	//
	// This object must fulfill the Sinkable contract.
	//
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	To *corev1.ObjectReference `json:"to,omitempty"`

	// Arguments is a list of configuration arguments for the Subscription. The
	// Arguments for a channel must contain values for each of the Parameters
	// specified by the Bus' spec.parameters.Subscriptions field except the
	// Parameters that have a default value. If a Parameter has a default value
	// and it is not in the list of Arguments, the default value will be used; a
	// Parameter without a default value that does not have an Argument will
	// result in an error setting up the Subscription.
	Arguments *[]Argument `json:"arguments,omitempty"`
}

type SubscriptionConditionType string

const (
	// Dispatching means the subscription is actively listening for incoming events on its channel and dispatching them.
	SubscriptionDispatching SubscriptionConditionType = "Dispatching"
)

// SubscriptionCondition describes the state of a subscription at a point in time.
type SubscriptionCondition struct {
	// Type of subscription condition.
	Type SubscriptionConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status v1.ConditionStatus `json:"status"`
	// The last time this condition was updated.
	LastUpdateTime meta_v1.Time `json:"lastUpdateTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime meta_v1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

// SubscriptionStatus (computed) for a subscription
type SubscriptionStatus struct {

	// Represents the latest available observations of a subscription's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []SubscriptionCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

func (ss *SubscriptionStatus) GetCondition(t SubscriptionConditionType) *SubscriptionCondition {
	for _, cond := range ss.Conditions {
		if cond.Type == t {
			return &cond
		}
	}
	return nil
}

func (s *Subscription) GetSpecJSON() ([]byte, error) {
	return json.Marshal(s.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionList returned in list operations
type SubscriptionList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Subscription `json:"items"`
}
