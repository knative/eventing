/*
 * Copyright 2018 The Knative Authors
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
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestGetFullType(t *testing.T) {
	s := &Subscribable{}
	switch s.GetFullType().(type) {
	case *Channel:
		// expected
	default:
		t.Errorf("expected GetFullType to return *Channel, got %T", s.GetFullType())
	}
}

func TestGetListType(t *testing.T) {
	c := &Channel{}
	switch c.GetListType().(type) {
	case *ChannelList:
		// expected
	default:
		t.Errorf("expected GetFullType to return *ChannelList, got %T", c.GetListType())
	}
}

func TestPopulate(t *testing.T) {
	got := &Channel{}

	want := &Channel{
		Spec: ChannelSpec{
			Subscribable: &Subscribable{
				Subscribers: []ChannelSubscriberSpec{{
					Ref: &corev1.ObjectReference{
						APIVersion: "eventing.knative.dev/v1alpha1",
						Kind:       "Subscription",
						Name:       "subscription1",
						Namespace:  "default",
						UID:        "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
					},
					SubscriberURI: "call1",
					ReplyURI:      "sink2",
				}, {
					Ref: &corev1.ObjectReference{
						APIVersion: "eventing.knative.dev/v1alpha1",
						Kind:       "Subscription",
						Name:       "subscription2",
						Namespace:  "default",
						UID:        "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
					},
					SubscriberURI: "call2",
					ReplyURI:      "sink2",
				}},
			},
		},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}

}
