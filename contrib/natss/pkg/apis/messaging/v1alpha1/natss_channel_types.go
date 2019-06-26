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
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/webhook"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NatssChannel is a resource representing a NATSS Channel.
type NatssChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec NatssChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the NatssChannel. This data may be out of
	// date.
	// +optional
	Status NatssChannelStatus `json:"status,omitempty"`
}

// Check that Channel can be validated, can be defaulted, and has immutable fields.
var _ apis.Validatable = (*NatssChannel)(nil)
var _ apis.Defaultable = (*NatssChannel)(nil)
var _ runtime.Object = (*NatssChannel)(nil)
var _ webhook.GenericCRD = (*NatssChannel)(nil)

// NatssChannelSpec defines the specification for a NatssChannel.
type NatssChannelSpec struct {
	// NatssChannel conforms to Duck type Subscribable.
	Subscribable *eventingduck.Subscribable `json:"subscribable,omitempty"`
}

// NatssChannelStatus represents the current state of a NatssChannel.
type NatssChannelStatus struct {
	// inherits duck/v1beta1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// NatssChannel is Addressable. It currently exposes the endpoint as a
	// fully-qualified DNS name which will distribute traffic over the
	// provided targets from inside the cluster.
	//
	// It generally has the form {channel}.{namespace}.svc.{cluster domain name}
	duckv1alpha1.AddressStatus `json:",inline"`

	// Subscribers is populated with the statuses of each of the Channelable's subscribers.
	eventingduck.SubscribableTypeStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NatssChannelList is a collection of NatssChannels.
type NatssChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NatssChannel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for NatssChannels
func (c *NatssChannel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("NatssChannel")
}
