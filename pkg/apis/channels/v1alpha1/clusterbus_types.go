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
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// ClusterBus represents the clusterbuses.channels.knative.dev CRD
type ClusterBus struct {
	meta_v1.TypeMeta   `json:",inline"`
	meta_v1.ObjectMeta `json:"metadata"`
	Spec               ClusterBusSpec    `json:"spec"`
	Status             *ClusterBusStatus `json:"status,omitempty"`
}

// ClusterBusSpec (what the user wants) for a clusterbus
type ClusterBusSpec struct {

	// Parameters exposed by the clusterbus for channels and subscriptions
	Parameters *ClusterBusParameters `json:"parameters,omitempty"`

	// Provisioner container definition to manage channels on the clusterbus.
	Provisioner *kapi.Container `json:"provisioner,omitempty"`

	// Dispatcher container definition to use for the clusterbus data plane.
	Dispatcher kapi.Container `json:"dispatcher"`

	// Volumes to be mounted inside the provisioner or dispatcher containers
	Volumes *[]kapi.Volume `json:"volumes,omitempty"`
}

// ClusterBusParameters parameters exposed by the clusterbus
type ClusterBusParameters struct {

	// Channel configuration params for channels on the clusterbus
	Channel *[]Parameter `json:"channel,omitempty"`

	// Subscription configuration params for subscriptions on the clusterbus
	Subscription *[]Parameter `json:"subscription,omitempty"`
}

// ClusterBusStatus (computed) for a clusterbus
type ClusterBusStatus struct {
}

func (b *ClusterBus) GetSpecJSON() ([]byte, error) {
	return json.Marshal(b.Spec)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterBusList returned in list operations
type ClusterBusList struct {
	meta_v1.TypeMeta `json:",inline"`
	meta_v1.ListMeta `json:"metadata"`
	Items            []ClusterBus `json:"items"`
}
