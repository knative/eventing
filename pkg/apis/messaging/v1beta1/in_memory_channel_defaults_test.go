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

package v1beta1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/messaging/config"

	"github.com/google/go-cmp/cmp"
)

func TestInMemoryChannelSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		channelTemplate *config.ChannelTemplateSpec
		initial         InMemoryChannel
		expected        InMemoryChannel
	}{
		"nil gets annotations": {
			initial:  InMemoryChannel{},
			expected: InMemoryChannel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1beta1"}}},
		},
		"empty gets annotations": {
			initial:  InMemoryChannel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}},
			expected: InMemoryChannel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1beta1"}}},
		},
		"non-empty gets added ChannelDefaulter": {
			initial:  InMemoryChannel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"somethingelse": "yup"}}},
			expected: InMemoryChannel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1beta1", "somethingelse": "yup"}}},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			tc.initial.SetDefaults(context.Background())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
