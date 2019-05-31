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
	// This is the list of subscriptions for this subscribable.
	// +patchMergeKey=uid
	// +patchStrategy=merge
	Subscribers []SubscriberSpec `json:"subscribers,omitempty" patchStrategy:"merge" patchMergeKey:"uid"`
}

// Subscribable is an Implementable "duck type".
var _ duck.Implementable = (*Subscribable)(nil)

// SubscriberSpec defines a single subscriber to a Subscribable.
// Ref is a reference to the Subscription this SubscriberSpec was created for
// SubscriberURI is the endpoint for the subscriber
// ReplyURI is the endpoint for the reply
// At least one of SubscriberURI and ReplyURI must be present
type SubscriberSpec struct {
	// Deprecated: use UID.
	// +optional
	DeprecatedRef *corev1.ObjectReference `json:"ref,omitempty" yaml:"ref,omitempty"`
	// UID is used to understand the origin of the subscriber.
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// +optional
	SubscriberURI string `json:"subscriberURI,omitempty"`
	// +optional
	ReplyURI string `json:"replyURI,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscribableType is a skeleton type wrapping Subscribable in the manner we expect resource writers
// defining compatible resources to embed it. We will typically use this type to deserialize
// SubscribableType ObjectReferences and access the Subscription data.  This is not a real resource.
type SubscribableType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SubscribableSpec is the part where Subscribable object is
	// configured as to be compatible with Subscribable contract.
	Spec SubscribableSpec `json:"spec"`
}

// SubscribableSpec shows how we expect folks to embed Subscribable in their Spec field.
type SubscribableSpec struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

var (
	// Verify SubscribableType resources meet duck contracts.
	_ duck.Populatable = (*SubscribableType)(nil)
	_ apis.Listable    = (*SubscribableType)(nil)
)

// GetFullType implements duck.Implementable
func (s *Subscribable) GetFullType() duck.Populatable {
	return &SubscribableType{}
}

// Populate implements duck.Populatable
func (c *SubscribableType) Populate() {
	c.Spec.Subscribable = &Subscribable{
		// Populate ALL fields
		Subscribers: []SubscriberSpec{{
			UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
			SubscriberURI: "call1",
			ReplyURI:      "sink2",
		}, {
			UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
			SubscriberURI: "call2",
			ReplyURI:      "sink2",
		}},
	}
}

// GetListType implements apis.Listable
func (c *SubscribableType) GetListType() runtime.Object {
	return &SubscribableTypeList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscribableTypeList is a list of SubscribableType resources
type SubscribableTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []SubscribableType `json:"items"`
}
