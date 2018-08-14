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
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// SubscriptionSpec specifies the Channel and Subscriber and the configuration
// arguments for the Subscription.
type SubscriptionSpec struct {
	// Channel is the name of the channel to subscribe to.
	Channel string `json:"channel"`

	// Subscriber is the name of the subscriber service DNS name.
	Subscriber string `json:"subscriber"`

	// Target service DNS name for replies returned by the subscriber.
	ReplyTo string `json:"replyTo,omitempty"`

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
