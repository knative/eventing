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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
)

func TestChannelableGetListType(t *testing.T) {
	c := &Channelable{}
	switch c.GetListType().(type) {
	case *ChannelableList:
		// expected
	default:
		t.Errorf("expected GetListType to return *ChannelableList, got %T", c.GetListType())
	}
}

func TestChannelablePopulate(t *testing.T) {
	got := &Channelable{}

	retry := int32(5)
	linear := BackoffPolicyLinear
	delay := "5s"
	want := &Channelable{
		Spec: ChannelableSpec{
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
			Delivery: &DeliverySpec{
				DeadLetterSink: &duckv1beta1.Destination{
					Ref: &corev1.ObjectReference{
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
		},

		Status: ChannelableStatus{
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
			SubscribableTypeStatus: SubscribableTypeStatus{
				SubscribableStatus: &SubscribableStatus{
					Subscribers: []SubscriberStatus{{
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
