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
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Subscription routes events received on a Channel to a DNS name and
// corresponds to the subscriptions.channels.knative.dev CRD.
type Subscription struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              SubscriptionSpec   `json:"spec"`
	Status            SubscriptionStatus `json:"status,omitempty"`
}

// Check that Subscription can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Subscription)(nil)
var _ apis.Defaultable = (*Subscription)(nil)
var _ apis.Immutable = (*Subscription)(nil)
var _ runtime.Object = (*Subscription)(nil)
var _ webhook.GenericCRD = (*Subscription)(nil)

// SubscriptionSpec specifies the Channel for incoming events, a Subscriber target
// for processing those events and where to put the result of the processing. Only
// From (where the events are coming from) is always required. You can optionally
// only Process the events (results in no output events) by leaving out the Result.
// You can also perform an identity transformation on the invoming events by leaving
// out the Subscriber and only specifying Result.
//
// The following are all valid specifications:
// channel --[subscriber]--> reply
// Sink, no outgoing events:
// channel -- subscriber
// no-op function (identity transformation):
// channel --> reply
type SubscriptionSpec struct {
	// TODO: Generation used to not work correctly with CRD. They were scrubbed
	// by the APIserver (https://github.com/kubernetes/kubernetes/issues/58778)
	// So, we add Generation here. Once the above bug gets rolled out to production
	// clusters, remove this and use ObjectMeta.Generation instead.
	// +optional
	Generation int64 `json:"generation,omitempty"`

	// Reference to a channel that will be used to create the subscription
	// for receiving events. The channel must have spec.subscriptions
	// list which will then be modified accordingly.
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// Kind must be "Channel" and APIVersion must be
	// "eventing.knative.dev/v1alpha1"
	//
	// This field is immutable. We have no good answer on what happens to
	// the events that are currently in the channel being consumed from
	// and what the semantics there should be. For now, you can always
	// delete the Subscription and recreate it to point to a different
	// channel, giving the user more control over what semantics should
	// be used (drain the channel first, possibly have events dropped,
	// etc.)
	Channel corev1.ObjectReference `json:"channel"`

	// Subscriber is reference to (optional) function for processing events.
	// Events from the Channel will be delivered here and replies are
	// sent to a channel as specified by the Reply.
	// +optional
	Subscriber *SubscriberSpec `json:"subscriber,omitempty"`

	// Reply specifies (optionally) how to handle events returned from
	// the Subscriber target.
	// +optional
	Reply *ReplyStrategy `json:"reply,omitempty"`
}

// SubscriberSpec specifies the reference to an object that's expected to
// provide the resolved target of the action.
// Currently we inspect the objects Status and see if there's a predefined
// Status field that we will then use to dispatch events to be processed by
// the target. Currently must resolve to a k8s service or Istio virtual
// service.
// Note that in the future we should try to utilize subresources (/resolve ?) to
// make this cleaner, but CRDs do not support subresources yet, so we need
// to rely on a specified Status field today. By relying on this behaviour
// we can utilize a dynamic client instead of having to understand all
// kinds of different types of objects. As long as they adhere to this
// particular contract, they can be used as a Target.
//
// This ensures that we can support external targets and for ease of use
// we also allow for an URI to be specified.
// There of course is also a requirement for the resolved SubscriberSpec to
// behave properly at the data plane level.
// TODO: Add a pointer to a real spec for this.
// For now, this means: Receive an event payload, and respond with one of:
// success and an optional response event, or failure.
// Delivery failures may be retried by the channel
type SubscriberSpec struct {
	// Only one of these can be specified

	// Reference to an object that will be used to find the target
	// endpoint, which should implement the Addressable duck type.
	// For example, this could be a reference to a Route resource
	// or a Knative Service resource.
	// TODO: Specify the required fields the target object must
	// have in the status.
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// +optional
	Ref *corev1.ObjectReference `json:"ref,omitempty"`

	// Reference to a 'known' endpoint where no resolving is done.
	// http://k8s-service for example
	// http://myexternalhandler.example.com/foo/bar
	// +optional
	DNSName *string `json:"dnsName,omitempty"`
}

// ReplyStrategy specifies the handling of the SubscriberSpec's returned replies.
// If no SubscriberSpec is specified, the identity function is assumed.
type ReplyStrategy struct {
	// This object must be a Channel.
	//
	// You can specify only the following fields of the ObjectReference:
	//   - Kind
	//   - APIVersion
	//   - Name
	// Kind must be "Channel" and APIVersion must be
	// "eventing.knative.dev/v1alpha1"
	// +optional
	Channel *corev1.ObjectReference `json:"channel,omitempty"`
}

// subCondSet is a condition set with Ready as the happy condition and
// ReferencesResolved and ChannelReady as the dependent conditions.
var subCondSet = duckv1alpha1.NewLivingConditionSet(SubscriptionConditionReferencesResolved, SubscriptionConditionChannelReady)

// SubscriptionStatus (computed) for a subscription
type SubscriptionStatus struct {
	// Represents the latest available observations of a subscription's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// PhysicalSubscription is the fully resolved values that this Subscription represents.
	PhysicalSubscription SubscriptionStatusPhysicalSubscription `json:"physicalSubscription,omitEmpty"`
}

// SubscriptionStatusPhysicalSubscription represents the fully resolved values for this
// Subscription.
type SubscriptionStatusPhysicalSubscription struct {
	// SubscriberURI is the fully resolved URI for spec.subscriber.
	SubscriberURI string `json:"subscriberURI,omitEmpty"`

	// ReplyURI is the fully resolved URI for the spec.reply.
	ReplyURI string `json:"replyURI,omitEmpty"`
}

const (
	// SubscriptionConditionReady has status True when all subconditions below have been set to True.
	SubscriptionConditionReady = duckv1alpha1.ConditionReady

	// SubscriptionConditionReferencesResolved has status True when all the specified references have been successfully
	// resolved.
	SubscriptionConditionReferencesResolved duckv1alpha1.ConditionType = "Resolved"

	// SubscriptionConditionChannelReady has status True when controller has successfully added a
	// subscription to the spec.channel resource.
	SubscriptionConditionChannelReady duckv1alpha1.ConditionType = "ChannelReady"
)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ss *SubscriptionStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return subCondSet.Manage(ss).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ss *SubscriptionStatus) IsReady() bool {
	return subCondSet.Manage(ss).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ss *SubscriptionStatus) InitializeConditions() {
	subCondSet.Manage(ss).InitializeConditions()
}

// MarkReferencesResolved sets the ReferencesResolved condition to True state.
func (ss *SubscriptionStatus) MarkReferencesResolved() {
	subCondSet.Manage(ss).MarkTrue(SubscriptionConditionReferencesResolved)
}

// MarkChannelReady sets the ChannelReady condition to True state.
func (ss *SubscriptionStatus) MarkChannelReady() {
	subCondSet.Manage(ss).MarkTrue(SubscriptionConditionChannelReady)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscriptionList returned in list operations
type SubscriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Subscription `json:"items"`
}
