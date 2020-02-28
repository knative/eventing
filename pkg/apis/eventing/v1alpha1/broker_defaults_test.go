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
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	configDefaultChannelTemplate = &config.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}

	defaultChannelTemplate = &messagingv1beta1.ChannelTemplateSpec{
		TypeMeta: v1.TypeMeta{
			APIVersion: SchemeGroupVersion.String(),
			Kind:       "InMemoryChannel",
		},
	}
)

func TestBrokerSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *config.ChannelTemplateSpec
		initial             Broker
		expected            Broker
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected:            Broker{},
		},
		"unset ChannelDefaulter": {
			expected: Broker{},
		},
		"set ChannelDefaulter": {
			channelTemplate: configDefaultChannelTemplate,
			expected: Broker{
				Spec: BrokerSpec{
					ChannelTemplate: defaultChannelTemplate,
				},
			},
		},
		"template already specified": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Broker{
				Spec: BrokerSpec{
					ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
			expected: Broker{
				Spec: BrokerSpec{
					ChannelTemplate: &messagingv1beta1.ChannelTemplateSpec{
						TypeMeta: v1.TypeMeta{
							APIVersion: SchemeGroupVersion.String(),
							Kind:       "OtherChannel",
						},
					},
				},
			},
		},
		"config already specified": {
			channelTemplate: configDefaultChannelTemplate,
			initial: Broker{
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "k",
						Namespace:  "ns",
						Name:       "k",
						APIVersion: "api",
					},
				},
			},
			expected: Broker{
				Spec: BrokerSpec{
					Config: &duckv1.KReference{
						Kind:       "k",
						Namespace:  "ns",
						Name:       "k",
						APIVersion: "api",
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
