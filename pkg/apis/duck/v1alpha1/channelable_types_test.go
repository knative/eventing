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
	"github.com/knative/pkg/apis/duck/v1alpha1"
	"testing"

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

	want := &Channelable{
		Spec: SubscribableSpec{
			Subscribable: &Subscribable{
				Subscribers: []SubscriberSpec{{
					UID:           "2f9b5e8e-deb6-11e8-9f32-f2801f1b9fd1",
					SubscriberURI: "call1",
					ReplyURI:      "sink2",
				}, {
					UID:           "34c5aec8-deb6-11e8-9f32-f2801f1b9fd1",
					SubscriberURI: "call2",
					ReplyURI:      "sink2",
				}},
			},
		},
		Status: v1alpha1.AddressStatus{
			Address: &v1alpha1.Addressable{
				// Populate ALL fields
				Hostname: "this is not empty",
			},
		},
	}

	got.Populate()

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}

}
