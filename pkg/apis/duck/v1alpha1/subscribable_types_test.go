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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestSubscribableGetFullType(t *testing.T) {
	s := &Subscribable{}
	switch s.GetFullType().(type) {
	case *SubscribableType:
		// expected
	default:
		t.Errorf("expected GetFullType to return *SubscribableType, got %T", s.GetFullType())
	}
}

func TestSubscribableGetListType(t *testing.T) {
	c := &SubscribableType{}
	switch c.GetListType().(type) {
	case *SubscribableTypeList:
		// expected
	default:
		t.Errorf("expected GetListType to return *SubscribableTypeList, got %T", c.GetListType())
	}
}

func TestSubscribablePopulate(t *testing.T) {
	got := &SubscribableType{}

	want := &SubscribableType{
		Spec: SubscribableTypeSpec{
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
		Status: SubscribableTypeStatus{
			SubscribableStatus: &SubscribableStatus{
				// Populate ALL fields
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
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}

}

func TestSubscribableTypeStatusHelperMethods(t *testing.T) {
	s := &SubscribableStatus{
		// Populate ALL fields
		Subscribers: []SubscriberStatus{{
			UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
			ObservedGeneration: 1,
			Ready:              corev1.ConditionTrue,
			Message:            "This is new field",
		}, {
			UID:                "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
			ObservedGeneration: 2,
			Ready:              corev1.ConditionFalse,
			Message:            "This is new field",
		}},
	}

	subscribableTypeStatus := SubscribableTypeStatus{
		SubscribableStatus: s,
	}

	/* Test GetSubscribableTypeStatus */

	// Should return SubscribableTypeStatus#SubscribableStatus
	subscribableStatus := subscribableTypeStatus.GetSubscribableTypeStatus()
	if subscribableStatus.Subscribers[0].Message != "This is new field" {
		t.Error("Testing of GetSubscribableTypeStatus failed as the function returned something unexpected")
	}

	/* Test SetSubscribableTypeStatus */

	// This should set both the fields to same value
	subscribableTypeStatus.SetSubscribableTypeStatus(*s)
	if subscribableTypeStatus.SubscribableStatus.Subscribers[0].Message != "This is new field" {
		t.Error("SetSubscribableTypeStatus didn't work as expected")
	}

	/* Test AddSubscriberToSubscribableStatus */
	subscribableTypeStatus.AddSubscriberToSubscribableStatus(SubscriberStatus{
		UID:                "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
		ObservedGeneration: 1,
		Ready:              corev1.ConditionTrue,
		Message:            "This is new field",
	})

	// Check if the subscriber was added to both the fields of SubscribableTypeStatus
	if len(subscribableTypeStatus.SubscribableStatus.Subscribers) != 3 {
		t.Error("AddSubscriberToSubscribableStatus didn't add subscriberstatus to both the fields of SubscribableTypeStatus")
	}
}
