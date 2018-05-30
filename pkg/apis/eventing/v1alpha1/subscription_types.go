/*
 * Copyright 2018 the original author or authors.
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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Represents the subscriptions.eventing.dev CRD
type Subscription struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               SubscriptionSpec    `json:"spec"`
	Status             *SubscriptionStatus `json:"status,omitempty"`
}

// Spec (what the user wants) for a subscription
type SubscriptionSpec struct {

	// Name of the stream to subscribe to
	Stream string `json:"stream"`

	// Name of the subscriber service
	Subscriber string `json:"subscriber"`

	// Parameters for the subscription
	Params *SubscriptionParams `json:"params,omitempty"`
}

// Params (key-value pairs) for a subscription
type SubscriptionParams map[string]string

// Status (computed) for a subscription
type SubscriptionStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Returned in list operations
type SubscriptionList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Subscription `json:"items"`
}
