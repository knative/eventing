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

	"knative.dev/eventing/pkg/apis/config"
	"knative.dev/eventing/pkg/apis/eventing"
	messagingconfig "knative.dev/eventing/pkg/apis/messaging/config"
	messagingv1beta1 "knative.dev/eventing/pkg/apis/messaging/v1beta1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	configDefaultChannelTemplate = &messagingconfig.ChannelTemplateSpec{
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
	defaultConfig = &config.Config{
		Defaults: &config.Defaults{
			// NamespaceDefaultsConfig are the default Broker Configs for each namespace.
			// Namespace is the key, the value is the KReference to the config.
			NamespaceDefaultsConfig: map[string]*config.ClassAndKRef{
				"mynamespace": {
					BrokerClass: "mynamespaceclass",
				},
			},
			ClusterDefault: &config.ClassAndKRef{
				BrokerClass: eventing.MTChannelBrokerClassValue,
			},
		},
	}
)

func TestBrokerSetDefaults(t *testing.T) {
	testCases := map[string]struct {
		nilChannelDefaulter bool
		channelTemplate     *messagingconfig.ChannelTemplateSpec
		initial             Broker
		expected            Broker
	}{
		"nil ChannelDefaulter": {
			nilChannelDefaulter: true,
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
		},
		"unset ChannelDefaulter": {
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
		},
		"set ChannelDefaulter": {
			channelTemplate: configDefaultChannelTemplate,
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
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
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
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
		"config already specified, adds annotation": {
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
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
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
		"class annotation exists": {
			initial: Broker{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "mynamespace",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "myclass",
					},
				},
			},
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "mynamespace",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "myclass",
					},
				},
			},
		},
		"default class annotation from cluster": {
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
		},
		"default class annotation from namespace": {
			initial: Broker{
				ObjectMeta: v1.ObjectMeta{Namespace: "mynamespace"},
			},
			expected: Broker{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "mynamespace",
					Annotations: map[string]string{
						eventing.BrokerClassKey: "mynamespaceclass",
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			ctx := context.Background()
			if !tc.nilChannelDefaulter {
				ctx = messagingconfig.ToContext(ctx, &messagingconfig.Config{
					ChannelDefaults: &messagingconfig.ChannelDefaults{
						ClusterDefault: tc.channelTemplate,
					},
				})
			}
			tc.initial.SetDefaults(config.ToContext(ctx, defaultConfig))
			if diff := cmp.Diff(tc.expected, tc.initial); diff != "" {
				t.Fatalf("Unexpected defaults (-want, +got): %s", diff)
			}
		})
	}
}
