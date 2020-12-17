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

package v1

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ChannelTemplateSpec struct {
	metav1.TypeMeta `json:",inline"`

	// Spec defines the Spec to use for each channel created. Passed
	// in verbatim to the Channel CRD as Spec section.
	// +optional
	Spec *runtime.RawExtension `json:"spec,omitempty"`
}

// ChannelTemplateSpecInternal is an internal only version that includes ObjectMeta so that
// we can easily create new Channels off of it.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ChannelTemplateSpecInternal struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec includes the Channel CR ChannelableSpec and the physical channel spec.
	// In order to create a new ChannelTemplateSpecInternalSpec, you must use NewChannelTemplateSpecInternalSpec
	Spec *ChannelTemplateSpecInternalSpec `json:"spec,omitempty"`
}

// ChannelTemplateSpecInternalSpec merges the "general" spec from Channel CR and the template of the physical channel spec.
// Note that this struct properly implements only Marshalling, unmarshalling doesn't work!
type ChannelTemplateSpecInternalSpec struct {
	// ChannelableSpec includes the fields from the Channel Spec section
	v1.ChannelableSpec

	// PhysicalChannelSpec includes the fields from the physical channel Spec. Passed
	// in verbatim to the Channel CRD as Spec section.
	// +optional
	PhysicalChannelSpec *runtime.RawExtension
}

// NewChannelTemplateSpecInternalSpec creates a new ChannelTemplateSpecInternalSpec, returning nil if channelableSpec is empty and physicalChannelSpec is nil.
func NewChannelTemplateSpecInternalSpec(channelableSpec v1.ChannelableSpec, physicalChannelSpec *runtime.RawExtension) *ChannelTemplateSpecInternalSpec {
	if physicalChannelSpec == nil && equality.Semantic.DeepEqual(channelableSpec, v1.ChannelableSpec{}) {
		return nil
	}
	return &ChannelTemplateSpecInternalSpec{
		ChannelableSpec:     channelableSpec,
		PhysicalChannelSpec: physicalChannelSpec,
	}
}

func (s ChannelTemplateSpecInternalSpec) MarshalJSON() ([]byte, error) {
	// Check if empty
	if s.PhysicalChannelSpec == nil && equality.Semantic.DeepEqual(s.ChannelableSpec, v1.ChannelableSpec{}) {
		return []byte{}, nil
	}

	// Let's merge the channel template spec and the channelable spec from channel
	channelableSpec := make(map[string]interface{})
	physicalChannelTemplateSpec := make(map[string]interface{})

	rawChannelSpec, err := json.Marshal(s.ChannelableSpec)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawChannelSpec, &channelableSpec); err != nil {
		return nil, err
	}

	if s.PhysicalChannelSpec != nil {
		rawPhysicalChannelTemplateSpec, err := json.Marshal(s.PhysicalChannelSpec)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(rawPhysicalChannelTemplateSpec, &physicalChannelTemplateSpec); err != nil {
			return nil, err
		}
	}

	// Merge the two maps into channelableSpec
	for k, v := range physicalChannelTemplateSpec {
		channelableSpec[k] = v
	}

	// Just return the merged map marshalled
	return json.Marshal(channelableSpec)
}
