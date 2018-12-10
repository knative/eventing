/*
Copyright 2018 The Knative Authors

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

package util

import (
	"testing"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

// These tests are here so that any changes made to the generation algorithm are noticed. Because
// the name is assumed to be stable and is calculated in both the controller and the dispatcher.
// TODO Generate the names in the controller and pass them through the Channel's status to the
// dispatcher.

func TestGenerateTopicName(t *testing.T) {
	expected := "knative-eventing-channel_channel-namespace_channel-name"
	actual := GenerateTopicName("channel-namespace", "channel-name")
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestGenerateSubName(t *testing.T) {
	expected := "knative-eventing-channel_sub-name_sub-uid"
	actual := GenerateSubName(&v1alpha1.ChannelSubscriberSpec{
		Ref: &v1.ObjectReference{
			Name: "sub-name",
			UID:  "sub-uid",
		},
	})
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}
