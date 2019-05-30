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

package channel

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// These tests are here so that any changes made to the generation algorithm are noticed. Because
// the name is assumed to be stable across controller runs.

func TestGenerateTopicName(t *testing.T) {
	expected := "knative-eventing-channel_channel-name_channel-uid"
	actual := generateTopicName(&eventingv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name: "channel-name",
			UID:  "channel-uid",
		},
	})
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}

func TestGenerateSubName(t *testing.T) {
	expected := "knative-eventing-channel_sub-name_sub-uid"
	actual := generateSubName(&v1alpha1.SubscriberSpec{
		DeprecatedRef: &v1.ObjectReference{
			Name: "sub-name",
			UID:  "sub-uid",
		},
		UID: "sub-uid",
	})
	if expected != actual {
		t.Errorf("Expected '%s'. Actual '%s'", expected, actual)
	}
}
