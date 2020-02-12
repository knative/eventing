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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InMemoryChannel is a resource representing an in memory channel
type InMemoryChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec InMemoryChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the Channel. This data may be out of
	// date.
	// +optional
	Status InMemoryChannelStatus `json:"status,omitempty"`
}

var (
	// Check that InMemoryChannel can be validated and defaulted.
	_ apis.Validatable = (*InMemoryChannel)(nil)
	_ apis.Defaultable = (*InMemoryChannel)(nil)

	// Check that InMemoryChannel can return its spec untyped.
	_ apis.HasSpec = (*InMemoryChannel)(nil)

	_ runtime.Object = (*InMemoryChannel)(nil)

	// Check that we can create OwnerReferences to an InMemoryChannel.
	_ kmeta.OwnerRefable = (*InMemoryChannel)(nil)
)

// InMemoryChannelSpec defines which subscribers have expressed interest in
// receiving events from this InMemoryChannel.
// arguments for a Channel.
type InMemoryChannelSpec struct {
	// Channel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

// ChannelStatus represents the current state of a Channel.
type InMemoryChannelStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// InMemoryChannel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1alpha1.AddressStatus `json:",inline"`

	// Subscribers is populated with the statuses of each of the Channelable's subscribers.
	eventingduck.SubscribableTypeStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InMemoryChannelList is a collection of in-memory channels.
type InMemoryChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InMemoryChannel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for InMemoryChannels
func (imc *InMemoryChannel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("InMemoryChannel")
}

// GetUntypedSpec returns the spec of the InMemoryChannel.
func (i *InMemoryChannel) GetUntypedSpec() interface{} {
	return i.Spec
}
