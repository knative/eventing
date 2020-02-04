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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// ApiServerSource is the Schema for the apiserversources API
type ApiServerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApiServerSourceSpec   `json:"spec,omitempty"`
	Status ApiServerSourceStatus `json:"status,omitempty"`
}

// Check the interfaces that ApiServerSource should be implementing.
var (
	_ runtime.Object     = (*ApiServerSource)(nil)
	_ kmeta.OwnerRefable = (*ApiServerSource)(nil)
	_ apis.Validatable   = (*ApiServerSource)(nil)
	_ apis.Defaultable   = (*ApiServerSource)(nil)
	_ apis.HasSpec       = (*ApiServerSource)(nil)
)

const (
	// ApiServerSourceAddEventType is the ApiServerSource CloudEvent type for adds.
	ApiServerSourceAddEventType = "dev.knative.apiserver.resource.add"
	// ApiServerSourceUpdateEventType is the ApiServerSource CloudEvent type for updates.
	ApiServerSourceUpdateEventType = "dev.knative.apiserver.resource.update"
	// ApiServerSourceDeleteEventType is the ApiServerSource CloudEvent type for deletions.
	ApiServerSourceDeleteEventType = "dev.knative.apiserver.resource.delete"

	// ApiServerSourceAddRefEventType is the ApiServerSource CloudEvent type for ref adds.
	ApiServerSourceAddRefEventType = "dev.knative.apiserver.ref.add"
	// ApiServerSourceUpdateRefEventType is the ApiServerSource CloudEvent type for ref updates.
	ApiServerSourceUpdateRefEventType = "dev.knative.apiserver.ref.update"
	// ApiServerSourceDeleteRefEventType is the ApiServerSource CloudEvent type for ref deletions.
	ApiServerSourceDeleteRefEventType = "dev.knative.apiserver.ref.delete"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApiServerSourceSpec defines the desired state of ApiServerSource
type ApiServerSourceSpec struct {
	// inherits duck/v1 SourceSpec, which currently provides:
	// * Sink - a reference to an object that will resolve to a domain name or
	//   a URI directly to use as the sink.
	// * CloudEventOverrides - defines overrides to control the output format
	//   and modifications of the event sent to the sink.
	duckv1.SourceSpec `json:",inline"`

	// TrackResource is the resource this source will track and send related
	// lifecycle events from the Kubernetes ApiServer.
	// +optional
	TrackResource *GroupVersionResourceKind `json:"resource,omitempty"`

	// LabelSelector filters this source to objects to those resources pass the
	// label selector.
	// More info: http://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	// +optional
	LabelSelector *metav1.LabelSelector `json:"selector,omitempty"`

	// ResourceOwners is an additional filter to only track resources that are
	// owned by a subset of controllers.
	// +optional
	ResourceOwners []GroupVersionResourceKind `json:"ownedBy,omitempty"`

	// EventMode controls the format of the event.
	// `Reference` sends a dataref event type for the resource under watch.
	// `Resource` send the full resource lifecycle event.
	// Defaults to `Reference`
	// +optional
	EventMode string `json:"mode,omitempty"`
}

// ApiServerSourceStatus defines the observed state of ApiServerSource
type ApiServerSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`
}

// GroupVersionResourceKind holds the the various ways to target resources inside of Kubernetes.
type GroupVersionResourceKind struct {
	// APIVersion - the API version of the resource to watch.
	// +optional
	APIVersion *string `json:"apiVersion"`

	// Group - the API group of the resource to watch.
	// +optional
	Group *string `json:"group"`

	// Version - the API version of the resource to watch.
	// +optional
	Version *string `json:"version"`

	// Kind of the resource to watch.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	// +optional
	Kind *string `json:"kind"`

	// Resource of the resource to watch.
	// +optional
	Resource *string `json:"resource"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApiServerSourceList contains a list of ApiServerSource
type ApiServerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiServerSource `json:"items"`
}
