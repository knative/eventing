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
	"knative.dev/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// PingSource is the Schema for the PingSources API.
type PingSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PingSourceSpec   `json:"spec,omitempty"`
	Status PingSourceStatus `json:"status,omitempty"`
}

// TODO: Check that PingSource can be validated and can be defaulted.

var (
	// Check that it is a runtime object.
	_ runtime.Object = (*PingSource)(nil)

	// Check that we can create OwnerReferences to a PingSource.
	_ kmeta.OwnerRefable = (*PingSource)(nil)

	// Check that PingSource can return its spec untyped.
	_ apis.HasSpec = (*PingSource)(nil)
)

type PingRequestsSpec struct {
	ResourceCPU    string `json:"cpu,omitempty"`
	ResourceMemory string `json:"memory,omitempty"`
}

type PingLimitsSpec struct {
	ResourceCPU    string `json:"cpu,omitempty"`
	ResourceMemory string `json:"memory,omitempty"`
}

type PingResourceSpec struct {
	Requests PingRequestsSpec `json:"requests,omitempty"`
	Limits   PingLimitsSpec   `json:"limits,omitempty"`
}

// PingSourceSpec defines the desired state of the PingSource.
type PingSourceSpec struct {
	// Schedule is the cronjob schedule.
	// +required
	Schedule string `json:"schedule"`

	// Data is the data posted to the target function.
	Data string `json:"data,omitempty"`

	// Sink is a reference to an object that will resolve to a uri to use as the sink.
	Sink *duckv1.Destination `json:"sink,omitempty"`

	// CloudEventOverrides defines overrides to control the output format and
	// modifications of the event sent to the sink.
	// +optional
	CloudEventOverrides *duckv1.CloudEventOverrides `json:"ceOverrides,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	// Deprecated: v1beta1 drops this field.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Resource limits and Request specifications of the Receive Adapter Deployment
	// Deprecated: v1beta1 drops this field.
	Resources PingResourceSpec `json:"resources,omitempty"`
}

// PingSourceStatus defines the observed state of PingSource.
type PingSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PingSourceList contains a list of PingSources.
type PingSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PingSource `json:"items"`
}
