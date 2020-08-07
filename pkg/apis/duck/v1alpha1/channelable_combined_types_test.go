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
	"testing"

	corev1 "k8s.io/api/core/v1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
)

func TestChannelableCombinedGetListType(t *testing.T) {
	c := &ChannelableCombined{}
	switch c.GetListType().(type) {
	case *ChannelableCombinedList:
		// expected
	default:
		t.Errorf("expected GetListType to return *ChannelableCombinedList, got %T", c.GetListType())
	}
}

func TestChannelableCombinedPopulate(t *testing.T) {
	got := &ChannelableCombined{}

	retry := int32(5)
	linear := eventingduckv1beta1.BackoffPolicyLinear
	linearv1 := eventingduckv1.BackoffPolicyLinear
	delay := "5s"
	want := &ChannelableCombined{
		Spec: ChannelableCombinedSpec{
			SubscribableSpec: eventingduckv1beta1.SubscribableSpec{
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
			},
			SubscribableSpecv1: eventingduckv1.SubscribableSpec{
				// Populate ALL fields
				Subscribers: []eventingduckv1.SubscriberSpec{{
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
			},
			SubscribableTypeSpec: SubscribableTypeSpec{
				Subscribable: &Subscribable{
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
				},
			},
			Delivery: &eventingduckv1beta1.DeliverySpec{
				DeadLetterSink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Name: "aname",
					},
					URI: &apis.URL{
						Scheme: "http",
						Host:   "test-error-domain",
					},
				},
				Retry:         &retry,
				BackoffPolicy: &linear,
				BackoffDelay:  &delay,
			},
			Deliveryv1: &eventingduckv1.DeliverySpec{
				DeadLetterSink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Name: "aname",
					},
					URI: &apis.URL{
						Scheme: "http",
						Host:   "test-error-domain",
					},
				},
				Retry:         &retry,
				BackoffPolicy: &linearv1,
				BackoffDelay:  &delay,
			},
		},

		Status: ChannelableCombinedStatus{
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
				Subscribers: []eventingduckv1beta1.SubscriberStatus{{
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
			},
			SubscribableStatusv1: eventingduckv1.SubscribableStatus{
				Subscribers: []eventingduckv1.SubscriberStatus{{
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
			},
			SubscribableTypeStatus: SubscribableTypeStatus{
				SubscribableStatus: &SubscribableStatus{
					Subscribers: []eventingduckv1beta1.SubscriberStatus{{
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
					Subscribersv1: []eventingduckv1.SubscriberStatus{{
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
				},
			},
		},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}

}
