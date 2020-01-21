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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/google/go-cmp/cmp"
)

var (
	defaultChannelTemplate = &ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}
)

func TestChannelSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *ChannelTemplateSpec
		initial             Channel
		expected            Channel
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Channel{},
		},
		"unset ChannelDefaulter": {
			expected: Channel{},
		},
		"set ChannelDefaulter": {
			channelTemplate: defaultChannelTemplate,
			expected: Channel{
				Spec: ChannelSpec{
					ChannelTemplate: defaultChannelTemplate,
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
			if !tc.nilChannelDefaulter {
				ChannelDefaulterSingleton = &channelDefaulter{
					channelTemplate: tc.channelTemplate,
				}
				defer func() { ChannelDefaulterSingleton = nil }()
			}
			tc.initial.SetDefaults(context.TODO())
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}

type channelDefaulter struct {
	channelTemplate *ChannelTemplateSpec
}

func (cd *channelDefaulter) GetDefault(_ string) *ChannelTemplateSpec {
	return cd.channelTemplate
}
