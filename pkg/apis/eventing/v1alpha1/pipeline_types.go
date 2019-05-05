/*
 * Copyright 2019 The Knative Authors
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
	"github.com/knative/pkg/apis"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Pipeline struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Pipeline.
	Spec PipelineSpec `json:"spec,omitempty"`

	// Status represents the current state of the Pipeline. This data may be out of
	// date.
	// +optional
	Status PipelineStatus `json:"status,omitempty"`
}

// Check that Pipeline can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*Pipeline)(nil)
var _ apis.Defaultable = (*Pipeline)(nil)
var _ apis.Immutable = (*Pipeline)(nil)
var _ runtime.Object = (*Pipeline)(nil)
var _ webhook.GenericCRD = (*Pipeline)(nil)

type PipelineSpec struct {
	// Steps is the list of Subscribers (processors / functions) that will be called in the order
	// provided.
	Steps []SubscriberSpec

	// Subscriber is the addressable that optionally receives events from the last step in the pipeline.
	// +optional
	Subscriber *SubscriberSpec `json:"subscriber,omitempty"`
}

type StepStatus struct {
	// Array of corresponding subscription statuses
	SubscriptionStatus []SubscriptionStatus
}

// PipelineStatus represents the current state of a Pipeline.
type PipelineStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// Addressable is the starting point to this Pipeline. Sending to this will target the first Subscriber.
	Address duckv1alpha1.Addressable `json:"address,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PipelineList is a collection of Pipelines.
type PipelineList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Pipeline `json:"items"`
}
