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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ApiServerSource is the Schema for the apiserversources API
type ApiServerSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ApiServerSourceSpec   `json:"spec,omitempty"`
	Status ApiServerSourceStatus `json:"status,omitempty"`
}

var (
	// Check that we can create OwnerReferences to an ApiServerSource.
	_ kmeta.OwnerRefable = (*ApiServerSource)(nil)

	// Check that ApiServerSource can return its spec untyped.
	_ apis.HasSpec = (*ApiServerSource)(nil)
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

// ApiServerSourceList contains a list of ApiServerSource
type ApiServerSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiServerSource `json:"items"`
}

// ApiServerSourceSpec defines the desired state of ApiServerSource
type ApiServerSourceSpec struct {
	// Resources is the list of resources to watch
	Resources []ApiServerResource `json:"resources"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this
	// source.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *duckv1beta1.Destination `json:"sink,omitempty"`

	// Mode is the mode the receive adapter controller runs under: Ref or Resource.
	// `Ref` sends only the reference to the resource.
	// `Resource` send the full resource.
	Mode string `json:"mode,omitempty"`
}

// ApiServerSourceStatus defines the observed state of ApiServerSource
type ApiServerSourceStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the ApiServerSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// ApiServerResource defines the resource to watch
type ApiServerResource struct {
	// API version of the resource to watch.
	APIVersion string `json:"apiVersion"`

	// Kind of the resource to watch.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`

	// LabelSelector restricts this source to objects with the selected labels
	// More info: http://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	LabelSelector metav1.LabelSelector `json:"labelSelector"`

	// ControllerSelector restricts this source to objects with a controlling owner reference of the specified kind.
	// Only apiVersion and kind are used. Both are optional.
	ControllerSelector metav1.OwnerReference `json:"controllerSelector"`

	// If true, send an event referencing the object controlling the resource
	Controller bool `json:"controller"`
}
