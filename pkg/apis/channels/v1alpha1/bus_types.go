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

	kapi "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// Bus represents the buses.channels.knative.dev CRD
type Bus struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               BusSpec    `json:"spec"`
	Status             *BusStatus `json:"status,omitempty"`
}

// BusSpec (what the user wants) for a bus
type BusSpec struct {

	// Parameters exposed by the bus for channels and subscriptions
	Parameters *BusParameters `json:"parameters,omitempty"`

	// Provisioner container definition to manage channels on the bus.
	Provisioner *kapi.Container `json:"provisioner,omitempty"`

	// Dispatcher container definition to use for the bus data plane.
	Dispatcher kapi.Container `json:"dispatcher"`

	// Volumes to be mounted inside the provisioner or dispatcher containers
	Volumes *[]kapi.Volume `json:"volumes,omitempty"`
}

// BusParameters parameters exposed by the bus
type BusParameters struct {

	// Channel configuration params for channels on the bus
	Channel *[]Parameter `json:"channel,omitempty"`

	// Subscription configuration params for subscriptions on the bus
	Subscription *[]Parameter `json:"subscription,omitempty"`
}

// BusStatus (computed) for a bus
type BusStatus struct {
}

func (b *Bus) GetSpecJSON() ([]byte, error) {
	return json.Marshal(b.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BusList returned in list operations
type BusList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []Bus `json:"items"`
}
