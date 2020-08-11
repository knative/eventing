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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
)

// +genduck

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
	// UID is used to understand the origin of the subscriber.
	// +optional
	UID types.UID `json:"uid,omitempty"`
	// Generation of the origin of the subscriber with uid:UID.
	// +optional
	Generation int64 `json:"generation,omitempty"`
	// +optional
	SubscriberURI *apis.URL `json:"subscriberURI,omitempty"`
	// +optional
	ReplyURI *apis.URL `json:"replyURI,omitempty"`
	// +optional
	DeadLetterSinkURI *apis.URL `json:"deadLetterSink,omitempty"`
	// +optional
	Delivery *duckv1beta1.DeliverySpec `json:"delivery,omitempty"`
	// +optional
	Deliveryv1 *duckv1.DeliverySpec `json:"deliveryv1,omitempty"`
}

// SubscribableStatus is the schema for the subscribable's status portion of the status
// section of the resource.
type SubscribableStatus struct {
	// This is the list of subscription's statuses for this channel.
	// +patchMergeKey=uid
	// +patchStrategy=merge
	Subscribers []duckv1beta1.SubscriberStatus `json:"subscribers,omitempty" patchStrategy:"merge" patchMergeKey:"uid"`

	// This is the list of subscription's statuses for this channel.
	// +patchMergeKey=uid
	// +patchStrategy=merge
	Subscribersv1 []duckv1.SubscriberStatus `json:"subscribersv1,omitempty" patchStrategy:"merge" patchMergeKey:"uid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubscribableType is a skeleton type wrapping Subscribable in the manner we expect resource writers
// defining compatible resources to embed it. We will typically use this type to deserialize
// SubscribableType ObjectReferences and access the Subscription data.  This is not a real resource.
type SubscribableType struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// SubscribableTypeSpec is the part where Subscribable object is
	// configured as to be compatible with Subscribable contract.
	Spec SubscribableTypeSpec `json:"spec"`

	// SubscribableTypeStatus is the part where SubscribableStatus object is
	// configured as to be compatible with Subscribable contract.
	Status SubscribableTypeStatus `json:"status"`
}

// SubscribableTypeSpec shows how we expect folks to embed Subscribable in their Spec field.
type SubscribableTypeSpec struct {
	Subscribable *Subscribable `json:"subscribable,omitempty"`
}

// SubscribableTypeStatus shows how we expect folks to embed Subscribable in their Status field.
type SubscribableTypeStatus struct {
	SubscribableStatus *SubscribableStatus `json:"subscribableStatus,omitempty"`
}

var (
	// Verify SubscribableType resources meet duck contracts.
	_ duck.Populatable = (*SubscribableType)(nil)
	_ apis.Listable    = (*SubscribableType)(nil)

	_ apis.Convertible = (*SubscribableType)(nil)

	_ apis.Convertible = (*SubscribableTypeSpec)(nil)
	_ apis.Convertible = (*SubscribableTypeStatus)(nil)

	_ apis.Convertible = (*SubscriberSpec)(nil)
)

// GetSubscribableTypeStatus method Returns the Default SubscribableStatus in this case it's SubscribableStatus
// This is w.r.t https://github.com/knative/eventing/pull/1685#discussion_r314797276
// Due to change in the API, we support reading of SubscribableTypeStatus#DeprecatedSubscribableStatus in a logical way
// where we read the V2 value first and if the value is absent then we read the V1 value,
// Having this function here makes it convinient to read the default value at runtime.
func (s *SubscribableTypeStatus) GetSubscribableTypeStatus() *SubscribableStatus {
	return s.SubscribableStatus
}

// SetSubscribableTypeStatus method sets the SubscribableStatus Values in th SubscribableTypeStatus structs
// This helper function ensures that we set both the values (SubscribableStatus and DeprecatedSubscribableStatus)
func (s *SubscribableTypeStatus) SetSubscribableTypeStatus(subscriberStatus SubscribableStatus) {
	s.SubscribableStatus = &subscriberStatus
}

// AddSubscriberToSubscribableStatus method is a Helper method for type SubscribableTypeStatus, if Subscribable Status needs to be appended
// with Subscribers, use this function, so that the value is reflected in both the duplicate fields residing
// in SubscribableTypeStatus
func (s *SubscribableTypeStatus) AddSubscriberToSubscribableStatus(subscriberStatus duckv1beta1.SubscriberStatus) {
	subscribers := append(s.GetSubscribableTypeStatus().Subscribers, subscriberStatus)
	s.SubscribableStatus.Subscribers = subscribers
}

// AddSubscriberToSubscribableStatus method is a Helper method for type SubscribableTypeStatus, if Subscribable Status needs to be appended
// with Subscribers, use this function, so that the value is reflected in both the duplicate fields residing
// in SubscribableTypeStatus
func (s *SubscribableTypeStatus) AddSubscriberV1ToSubscribableStatus(subscriberStatus duckv1.SubscriberStatus) {
	subscribersv1 := append(s.GetSubscribableTypeStatus().Subscribersv1, subscriberStatus)
	s.SubscribableStatus.Subscribersv1 = subscribersv1
}

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
			Generation:    1,
			SubscriberURI: apis.HTTP("call1"),
			ReplyURI:      apis.HTTP("sink2"),
		}, {
			UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
			Generation:    2,
			SubscriberURI: apis.HTTP("call2"),
			ReplyURI:      apis.HTTP("sink2"),
		}},
	}
	c.Status.SetSubscribableTypeStatus(SubscribableStatus{
		// Populate ALL fields
		Subscribers: []duckv1beta1.SubscriberStatus{{
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
		Subscribersv1: []duckv1.SubscriberStatus{{
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
	})
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
