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
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// Subscribable is the schema for the subscribable portion of the spec
// section of the resource.
type Subscribable struct {
	// TODO: What is actually required here for Channel spec.
	// This is the list of subscriptions for this channel.
	// +patchMergeKey=uid
	// +patchStrategy=merge
	Subscribers []ChannelSubscriberSpec `json:"subscribers,omitempty" patchStrategy:"merge" patchMergeKey:"uid"`
}

// Subscribable is an Implementable "duck type".
var _ duck.Implementable = (*Subscribable)(nil)

// ChannelSubscriberSpec defines a single subscriber to a Channel.
// Ref (Deprecated) is a reference to the Subscription this ChannelSubscriberSpec was created for
// SubscriberURI is the endpoint for the subscriber
// ReplyURI is the endpoint for the reply
// At least one of SubscriberURI and ReplyURI must be present
type ChannelSubscriberSpec struct {
	// Deprecated: use UID.
	// +optional
	DeprecatedRef *corev1.ObjectReference `json:"ref,omitempty" yaml:"ref,omitempty"`
	// UID is used to understand the origin of the subscriber.
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// Generation of the origin of the subscriber with uid:UID.
	// +optional
	Generation int64 `json:"generation,omitempty"`
	// +optional
	SubscriberURI string `json:"subscriberURI,omitempty"`
	// +optional
	ReplyURI string `json:"replyURI,omitempty"`
}

// SubscribableStatus is the schema for the subscribable's status portion of the status
// section of the resource.
type SubscribableStatus struct {
	// This is the list of subscription's statuses for this channel.
	// +patchMergeKey=uid
	// +patchStrategy=merge
	Subscribers []ChannelSubscriberStatus `json:"subscribers,omitempty" patchStrategy:"merge" patchMergeKey:"uid"`
}

// ChannelSubscriberStatus defines the status of a single subscriber to a Channel.
type ChannelSubscriberStatus struct {
	// UID is used to understand the origin of the subscriber.
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// Generation of the origin of the subscriber with uid:UID.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Status of the subscriber.
	// +optional
	Ready corev1.ConditionStatus `json:"ready,omitempty"`
	// A human readable message indicating details of Ready status.
	// +optional
	Message string `json:"message,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Channel is a skeleton type wrapping Subscribable in the manner we expect resource writers
// defining compatible resources to embed it. We will typically use this type to deserialize
// Channel ObjectReferences and access the Subscription data.  This is not a real resource.
type Channel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// ChannelSpec is the part where Subscribable object is
	// configured as to be compatible with Subscribable contract.
	Spec ChannelSpec `json:"spec"`

	// ChannelStatus is the part where SubscribableStatus object is
	// configured as to be compatible with Subscribable contract.
	Status ChannelStatus `json:"status"`
}

// ChannelSpec shows how we expect folks to embed Subscribable in their Spec field.
type ChannelSpec struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

// ChannelStatus shows how we expect folks to embed SubscribableStatus in their Status field.
type ChannelStatus struct {
	SubscribableStatus *SubscribableStatus `json:"subscribablestatus,omitempty"`
}

var (
	// Verify Channel resources meet duck contracts.
	_ duck.Populatable = (*Channel)(nil)
	_ apis.Listable    = (*Channel)(nil)
)

// GetFullType implements duck.Implementable
func (s *Subscribable) GetFullType() duck.Populatable {
	return &Channel{}
}

// Populate implements duck.Populatable
func (c *Channel) Populate() {
	c.Spec.Subscribable = &Subscribable{
		// Populate ALL fields
		Subscribers: []ChannelSubscriberSpec{{
			UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
			Generation:    1,
			SubscriberURI: "call1",
			ReplyURI:      "sink2",
		}, {
			UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
			Generation:    2,
			SubscriberURI: "call2",
			ReplyURI:      "sink2",
		}},
	}
	c.Status.SubscribableStatus = &SubscribableStatus{
		// Populate ALL fields
		Subscribers: []ChannelSubscriberStatus{{
			UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
			ObservedGeneration: 1,
			Ready:              corev1.ConditionTrue,
			Message:            "Some message",
		}, {
			UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
			ObservedGeneration: 2,
			Ready:              corev1.ConditionFalse,
			Message:            "Some message",
		}},
	}
}

// GetListType implements apis.Listable
func (c *Channel) GetListType() runtime.Object {
	return &ChannelList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelList is a list of Channel resources
type ChannelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Channel `json:"items"`
}
