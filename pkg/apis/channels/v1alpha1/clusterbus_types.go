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
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	Spec               ClusterBusSpec   `json:"spec"`
	Status             ClusterBusStatus `json:"status,omitempty"`
}

// Check that Bus can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*ClusterBus)(nil)
var _ apis.Defaultable = (*ClusterBus)(nil)
var _ apis.Immutable = (*ClusterBus)(nil)
var _ runtime.Object = (*ClusterBus)(nil)
var _ webhook.GenericCRD = (*ClusterBus)(nil)

// ClusterBusSpec (what the user wants) for a clusterbus
type ClusterBusSpec = BusSpec

// ClusterBusStatus (computed) for a clusterbus
type ClusterBusStatus = BusStatus

func (b *ClusterBus) BacksChannel(channel *Channel) bool {
	return len(b.Namespace) == 0 && b.Name == channel.Spec.ClusterBus
}

func (b *ClusterBus) GetSpec() *ClusterBusSpec {
	return &b.Spec
}

func (b *ClusterBus) GetStatus() *ClusterBusStatus {
	return &b.Status
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
