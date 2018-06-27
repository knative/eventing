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

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Subscription represents the subscriptions.channels.knative.dev CRD
type Subscription struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               SubscriptionSpec   `json:"spec"`
	Status             SubscriptionStatus `json:"status,omitempty"`
}

// SubscriptionSpec (what the user wants) for a subscription
type SubscriptionSpec struct {

	// Name of the channel to subscribe to
	Channel string `json:"channel"`

	// Name of the subscriber service
	Subscriber string `json:"subscriber"`

	// Arguments for the subscription
	Arguments *[]Argument `json:"arguments,omitempty"`
}

// SubscriptionStatus (computed) for a subscription
type SubscriptionStatus struct {
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
