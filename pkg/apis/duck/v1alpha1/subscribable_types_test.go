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
					SubscriberURI: "call1",
					ReplyURI:      "sink2",
				}, {
					UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
					Generation:    2,
					SubscriberURI: "call2",
					ReplyURI:      "sink2",
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
