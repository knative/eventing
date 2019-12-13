/*
Copyright 2019 The Knative Authors

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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// SinkBinding describes a Binding that is also a Source.
// The `sink` (from the Source duck) is resolved to a URL and
// then projected into the `subject` by augmenting the runtime
// contract of the referenced containers to have a `K_SINK`
// environment variable holding the endpoint to which to send
// cloud events.
type SinkBinding v1alpha1.SinkBinding

var (
	_ kmeta.OwnerRefable = (*SinkBinding)(nil)
)

// GetGroupVersionKind returns the GroupVersionKind.
func (s *SinkBinding) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SinkBinding")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SinkBindingList contains a list of SinkBinding
type SinkBindingList v1alpha1.SinkBindingList
