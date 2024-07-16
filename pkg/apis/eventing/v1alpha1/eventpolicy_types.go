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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventPolicy represents a policy for addressable resources (Broker, Channel, sinks).
type EventPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the EventPolicy.
	Spec EventPolicySpec `json:"spec,omitempty"`

	// Status represents the current state of the EventPolicy.
	// This data may be out of date.
	// +optional
	Status EventPolicyStatus `json:"status,omitempty"`
}

var (
	// Check that EventPolicy can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*EventPolicy)(nil)
	_ apis.Defaultable = (*EventPolicy)(nil)

	// Check that EventPolicy can return its spec untyped.
	_ apis.HasSpec = (*EventPolicy)(nil)

	_ runtime.Object = (*EventPolicy)(nil)

	// Check that we can create OwnerReferences to an EventPolicy.
	_ kmeta.OwnerRefable = (*EventPolicy)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*EventPolicy)(nil)
)

type EventPolicySpec struct {
	// To lists all resources for which this policy applies.
	// Resources in this list must act like an ingress and have an audience.
	// The resources are part of the same namespace as the EventPolicy.
	// An empty list means it applies to all resources in the EventPolicies namespace
	// +optional
	To []EventPolicySpecTo `json:"to,omitempty"`

	// From is the list of sources or oidc identities, which are allowed to send events to the targets (.spec.to).
	From []EventPolicySpecFrom `json:"from,omitempty"`
}

type EventPolicySpecTo struct {
	// Ref contains the direct reference to a target
	// +optional
	Ref *EventPolicyToReference `json:"ref,omitempty"`

	// Selector contains a selector to group targets
	// +optional
	Selector *EventPolicySelector `json:"selector,omitempty"`
}

type EventPolicySpecFrom struct {
	// Ref contains a direct reference to a resource which is allowed to send events to the target.
	// +optional
	Ref *EventPolicyFromReference `json:"ref,omitempty"`

	// Sub sets the OIDC identity name to be allowed to send events to the target.
	// It is also possible to set a glob-like pattern to match any suffix.
	// +optional
	Sub *string `json:"sub,omitempty"`
}

type EventPolicyToReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`

	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
}

type EventPolicyFromReference struct {
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`

	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`

	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`

	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	// This is optional field, it gets defaulted to the object holding it if left out.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type EventPolicySelector struct {
	*metav1.LabelSelector `json:",inline"`
	*metav1.TypeMeta      `json:",inline"`
}

// EventPolicyStatus represents the current state of a EventPolicy.
type EventPolicyStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// From is the list of resolved oidc identities from .spec.from
	From []string `json:"from,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventPolicyList is a collection of EventPolicy.
type EventPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventPolicy `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for EventPolicy
func (ep *EventPolicy) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("EventPolicy")
}

// GetUntypedSpec returns the spec of the EventPolicy.
func (ep *EventPolicy) GetUntypedSpec() interface{} {
	return ep.Spec
}

// GetStatus retrieves the status of the EventPolicy. Implements the KRShaped interface.
func (ep *EventPolicy) GetStatus() *duckv1.Status {
	return &ep.Status.Status
}
