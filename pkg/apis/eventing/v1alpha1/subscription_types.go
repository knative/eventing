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
var _ webhook.GenericCRD = (*Subscription)(nil)

// SubscriptionSpec specifies the Channel for incoming events, a Call target for
// processing those events and where to put the result of the processing. Only
// From (where the events are coming from) is always required. You can optionally
// only Process the events (results in no output events) by leaving out the Result.
// You can also perform an identity transformation on the invoming events by leaving
// out the Call and only specifying Result.
//
// The following are all valid specifications:
// from --[call]--> result
// Sink, no outgoing events:
// from -- call
// no-op function (identity transformation):
// from --> result
type SubscriptionSpec struct {
	// TODO: Generation used to not work correctly with CRD. They were scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once the above bug gets rolled out to production
	// clusters, remove this and use ObjectMeta.Generation instead.
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
	// This field is immutable
	From *corev1.ObjectReference `json:"from,omitempty"`

	// Call is reference to (optional) function for processing events.
	// Events from the From channel will be delivered here and replies
	// are sent to a channel as specified by the Result.
	// +optional
	Call *Callable `json:"call,omitempty"`

	// Result specifies (optionally) how to handle events returned from
	// the Call target.
	// +optional
	Result *ResultStrategy `json:"to,omitempty"`
}

// Callable specifies the reference to an object that's expected to
// provide the resolved target of the action. Currently we inspect
// the objects Status and see if there's a predefined Status field
// that we will then use to dispatch events to be processed by the target.
// Currently must resolve to a k8s service or Istio virtual service. Note that
// in the future we should try to utilize subresources (/resolve ?) to
// make this cleaner, but CRDs do not support subresources yet, so we need
// to rely on a specified Status field today. By relying on this behaviour
// we can utilize a dynamic client instead of having to understand all
// kinds of different types of objects. As long as they adhere to this
// particular contract, they can be used as a Target.
// This ensures that we can support external targets and for ease of use
// we also allow for an URI to be specified.
// There of course is also a requirement for the resolved Callable to
// behave properly at the data plane level.
type Callable struct {
	// Only one of these can be specified

	// Reference to an object that will be used to find the target
	// endpoint.
	// For example, this could be a reference to a Route resource
	// or a Knative Service resource.
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	Target *corev1.ObjectReference `json:"target,omitempty"`

	// Reference to a 'known' endpoint where no resolving is done.
	// http://k8s-service for example
	// http://myexternalhandler.example.com/foo/bar
	// +optional
	TargetURI *string `json:"targetURI,omitempty"`
}

type ResultStrategy struct {
	// This object must fulfill the Sinkable contract.
	//
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	Target *corev1.ObjectReference `json:"target,omitempty"`
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
