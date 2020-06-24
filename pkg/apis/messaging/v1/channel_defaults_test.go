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

package v1

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/messaging/config"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultChannelTemplate = &config.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}
	ourDefaultChannelTemplate = &ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}
)

func TestChannelSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *config.ChannelTemplateSpec
		initial             Channel
		expected            Channel
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Channel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1"}}},
		},
		"unset ChannelDefaulter": {
			expected: Channel{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1"}}},
		},
		"set ChannelDefaulter": {
			channelTemplate: defaultChannelTemplate,
			expected: Channel{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1"}},
				Spec: ChannelSpec{
					ChannelTemplate: ourDefaultChannelTemplate,
				},
			},
		},
		"template already specified": {
			channelTemplate: defaultChannelTemplate,
			initial: Channel{
				Spec: ChannelSpec{
					ChannelTemplate: &ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
			expected: Channel{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"messaging.knative.dev/subscribable": "v1"}},
				Spec: ChannelSpec{
					ChannelTemplate: &ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.Background()
			if !tc.nilChannelDefaulter {
				ctx = config.ToContext(ctx, &config.Config{
					ChannelDefaults: &config.ChannelDefaults{
						ClusterDefault: tc.channelTemplate,
					},
				})
			}
			tc.initial.SetDefaults(ctx)
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
