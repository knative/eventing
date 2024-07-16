/*
Copyright 2021 The Knative Authors

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

package duck

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// channelTemplateSpecInternal is an internal only version that includes ObjectMeta so that
// we can easily create new Channels off of it.
type channelTemplateSpecInternal struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec includes the Channel CR ChannelableSpec and the physical channel spec.
	// In order to create a new channelTemplateSpecInternalSpec, you must use NewChannelTemplateSpecInternalSpec
	// +optional
	Spec *channelTemplateSpecInternalSpec `json:"spec,omitempty"`
}

// channelTemplateSpecInternalSpec merges the "general" spec from Channel CR and the template of the physical channel spec.
// Note that this struct properly implements only Marshalling, unmarshalling doesn't work!
type channelTemplateSpecInternalSpec struct {
	// channelableSpec includes the fields from the Channel Spec section
	channelableSpec *duckv1.ChannelableSpec

	// physicalChannelSpec includes the fields from the physical channel Spec. Passed
	// in verbatim to the Channel CRD as Spec section.
	// +optional
	physicalChannelSpec *runtime.RawExtension
}

func (s *channelTemplateSpecInternalSpec) MarshalJSON() ([]byte, error) {
	// Let's merge the channel template spec and the channelable spec from channel
	channelableSpec := make(map[string]interface{})
	physicalChannelTemplateSpec := make(map[string]interface{})

	var cs duckv1.ChannelableSpec
	if s.channelableSpec != nil {
		cs = *s.channelableSpec
	} else {
		cs = duckv1.ChannelableSpec{}
	}

	rawChannelSpec, err := json.Marshal(cs)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(rawChannelSpec, &channelableSpec); err != nil {
		return nil, err
	}

	if s.physicalChannelSpec != nil {
		rawPhysicalChannelTemplateSpec, err := json.Marshal(s.physicalChannelSpec)
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

// PhysicalChannelOption represents an option for NewPhysicalChannel.
type PhysicalChannelOption func(*channelTemplateSpecInternal)

// WithChannelableSpec sets the ChannelableSpec of the physical channel.
func WithChannelableSpec(channelableSpec duckv1.ChannelableSpec) PhysicalChannelOption {
	return func(internal *channelTemplateSpecInternal) {
		if equality.Semantic.DeepEqual(channelableSpec, duckv1.ChannelableSpec{}) {
			// No need to set it
			return
		}
		if internal.Spec == nil {
			internal.Spec = &channelTemplateSpecInternalSpec{}
		}
		internal.Spec.channelableSpec = &channelableSpec
	}
}

// WithPhysicalChannelSpec sets the ChannelableSpec of the physical channel.
func WithPhysicalChannelSpec(physicalChannelSpec *runtime.RawExtension) PhysicalChannelOption {
	return func(internal *channelTemplateSpecInternal) {
		if physicalChannelSpec == nil {
			// No need to set it
			return
		}
		if internal.Spec == nil {
			internal.Spec = &channelTemplateSpecInternalSpec{}
		}
		internal.Spec.physicalChannelSpec = physicalChannelSpec
	}
}

// NewPhysicalChannel returns a new physical channel as unstructured.Unstructured, starting from the provided meta and ducks.
// When developing components that needs to spawn underlying (physical) channels (e.g. the Channel type, brokers backed by channels, parallel, sequence)
// you should use this function to create the underlying channels.
// Any physical channel CRD is composed by TypeMeta, ObjectMeta and a Spec, which includes
// a ChannelableSpec and optionally additional fields (e.g. like KafkaChannel includes partitionCount).
// You can set the ChannelableSpec using WithChannelableSpec and you can set all the additional fields using WithPhysicalChannelSpec
// This function returns the channel with the provided TypeMeta and ObjectMeta and with the additional provided options.
//
// For example, providing the TypeMeta of an InMemoryChannel and a ChannelableSpec with delivery configured, the return value will look like:
//
// apiVersion: messaging.knative.dev/v1
// kind: InMemoryChannel
// metadata:
//
//	name: "hello"
//	namespace: "world"
//
// spec:
//
//	delivery:
//	  retry: 3
func NewPhysicalChannel(typeMeta metav1.TypeMeta, objMeta metav1.ObjectMeta, opts ...PhysicalChannelOption) (*unstructured.Unstructured, error) {
	// Set the name of the resource we're creating as well as the namespace, etc.
	template := channelTemplateSpecInternal{
		TypeMeta:   typeMeta,
		ObjectMeta: objMeta,
	}

	for _, opt := range opts {
		opt(&template)
	}

	raw, err := json.Marshal(template)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = json.Unmarshal(raw, u)
	if err != nil {
		return nil, err
	}
	return u, nil
}
