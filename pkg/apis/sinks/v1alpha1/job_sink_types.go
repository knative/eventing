/*
Copyright 2024 The Knative Authors

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
	batchv1 "k8s.io/api/batch/v1"
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

// JobSink is the Schema for the JobSink API.
type JobSink struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JobSinkSpec   `json:"spec,omitempty"`
	Status JobSinkStatus `json:"status,omitempty"`
}

// Check the interfaces that JobSink should be implementing.
var (
	_ runtime.Object     = (*JobSink)(nil)
	_ kmeta.OwnerRefable = (*JobSink)(nil)
	_ apis.Validatable   = (*JobSink)(nil)
	_ apis.Defaultable   = (*JobSink)(nil)
	_ apis.HasSpec       = (*JobSink)(nil)
	_ duckv1.KRShaped    = (*JobSink)(nil)
)

// JobSinkSpec defines the desired state of the JobSink.
type JobSinkSpec struct {
	// Job to run when an event occur.
	// +optional
	Job *batchv1.Job `json:"job,omitempty"`
}

// JobSinkStatus defines the observed state of JobSink.
type JobSinkStatus struct {
	duckv1.Status `json:",inline"`

	// AddressStatus is the part where the JobSink fulfills the Addressable contract.
	// It exposes the endpoint as an URI to get events delivered.
	// +optional
	duckv1.AddressStatus `json:",inline"`

	// +optional
	JobStatus JobStatus `json:"job,omitempty"`
}

type JobStatus struct {
	Selector string `json:"selector,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JobSinkList contains a list of JobSink.
type JobSinkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JobSink `json:"items"`
}

// GetStatus retrieves the status of the JobSink. Implements the KRShaped interface.
func (sink *JobSink) GetStatus() *duckv1.Status {
	return &sink.Status.Status
}
