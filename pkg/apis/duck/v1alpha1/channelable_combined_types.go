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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// +genduck
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelableCombined is a skeleton type wrapping Subscribable and Addressable of
// v1alpha1 and v1beta1 duck types. This is not to be used by resource writers and is
// only used by Subscription Controller to synthesize patches and read the Status
// of the Channelable Resources.
// This is not a real resource.
type ChannelableCombined struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the part where the Channelable fulfills the Subscribable contract.
	Spec ChannelableCombinedSpec `json:"spec,omitempty"`

	Status ChannelableCombinedStatus `json:"status,omitempty"`
}

// ChannelableSpec contains Spec of the Channelable object
type ChannelableCombinedSpec struct {
	// SubscribableTypeSpec is for the v1alpha1 spec compatibility.
	SubscribableTypeSpec `json:",inline"`

	// SubscribableSpec is for the v1beta1 spec compatibility.
	eventingduckv1beta1.SubscribableSpec `json:",inline"`

	// DeliverySpec contains options controlling the event delivery
	// +optional
	Delivery *eventingduckv1beta1.DeliverySpec `json:"delivery,omitempty"`
}

// ChannelableStatus contains the Status of a Channelable object.
type ChannelableCombinedStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
	// AddressStatus is the part where the Channelable fulfills the Addressable contract.
	v1alpha1.AddressStatus `json:",inline"`
	// SubscribableTypeStatus is the v1alpha1 part of the Subscribers status
	SubscribableTypeStatus `json:",inline"`
	// SubscribableStatus is the v1beta1 part of the Subscribers status.
	eventingduckv1beta1.SubscribableStatus `json:",inline"`
	// ErrorChannel is set by the channel when it supports native error handling via a channel
	// +optional
	ErrorChannel *corev1.ObjectReference `json:"errorChannel,omitempty"`
}

var (
	// Verify Channelable resources meet duck contracts.
	_ duck.Populatable   = (*ChannelableCombined)(nil)
	_ duck.Implementable = (*ChannelableCombined)(nil)
	_ apis.Listable      = (*ChannelableCombined)(nil)
)

// Populate implements duck.Populatable
func (c *ChannelableCombined) Populate() {
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
	c.Spec.SubscribableSpec = eventingduckv1beta1.SubscribableSpec{
		// Populate ALL fields
		Subscribers: []eventingduckv1beta1.SubscriberSpec{{
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
	retry := int32(5)
	linear := eventingduckv1beta1.BackoffPolicyLinear
	delay := "5s"
	deadLetterSink := duckv1.Destination{
		Ref: &duckv1.KReference{
			Name: "aname",
		},
		URI: &apis.URL{
			Scheme: "http",
			Host:   "test-error-domain",
		},
	}
	c.Spec.Delivery = &eventingduckv1beta1.DeliverySpec{
		DeadLetterSink: &deadLetterSink,
		Retry:          &retry,
		BackoffPolicy:  &linear,
		BackoffDelay:   &delay,
	}
	subscribers := []eventingduckv1beta1.SubscriberStatus{{
		UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 1,
		Ready:              corev1.ConditionTrue,
		Message:            "Some message",
	}, {
		UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 2,
		Ready:              corev1.ConditionFalse,
		Message:            "Some message",
	}}
	c.Status = ChannelableCombinedStatus{
		AddressStatus: v1alpha1.AddressStatus{
			Address: &v1alpha1.Addressable{
				// Populate ALL fields
				Addressable: duckv1beta1.Addressable{
					URL: &apis.URL{
						Scheme: "http",
						Host:   "test-domain",
					},
				},
				Hostname: "test-domain",
			},
		},
		SubscribableStatus: eventingduckv1beta1.SubscribableStatus{
			Subscribers: subscribers,
		},
		SubscribableTypeStatus: SubscribableTypeStatus{
			SubscribableStatus: &SubscribableStatus{
				Subscribers: subscribers,
			},
		},
	}
}

// GetFullType implements duck.Implementable
func (s *ChannelableCombined) GetFullType() duck.Populatable {
	return &ChannelableCombined{}
}

// GetListType implements apis.Listable
func (c *ChannelableCombined) GetListType() runtime.Object {
	return &ChannelableCombinedList{}
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ChannelableList is a list of Channelable resources.
type ChannelableCombinedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []ChannelableCombined `json:"items"`
}
