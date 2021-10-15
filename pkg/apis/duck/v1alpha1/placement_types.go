/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis/duck"
)

// +genduck

// Placeable is a list of podName and virtual replicas pairs.
// Each pair represents the assignment of virtual replicas to a pod
type Placeable struct {
	MaxAllowedVReplicas *int32      `json:"maxAllowedVReplicas,omitempty"`
	Placements          []Placement `json:"placements,omitempty"`
}

// PlaceableType is a skeleton type wrapping Placeable in the manner we expect
// resource writers defining compatible resources to embed it.  We will
// typically use this type to deserialize Placeable ObjectReferences and
// access the Placeable data.  This is not a real resource.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PlaceableType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status PlaceableStatus `json:"status"`
}

type PlaceableStatus struct {
	Placeable `json:",inline"`
}

type Placement struct {
	// PodName is the name of the pod where the resource is placed
	PodName string `json:"podName,omitempty"`

	// VReplicas is the number of virtual replicas assigned to in the pod
	VReplicas int32 `json:"vreplicas,omitempty"`
}

var (
	// Placeable is an Implementable "duck type".
	_ duck.Implementable = (*Placeable)(nil)
)

// GetFullType implements duck.Implementable
func (*Placeable) GetFullType() duck.Populatable {
	return &PlaceableType{}
}

// Populate implements duck.Populatable
func (t *PlaceableType) Populate() {
	t.Status.Placements = []Placement{{PodName: "pod-0", VReplicas: int32(1)}}
}

// GetListType implements apis.Listable
func (*PlaceableType) GetListType() runtime.Object {
	return &PlaceableList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlaceableList is a list of PlaceableType resources
type PlaceableList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Placeable `json:"items"`
}
