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

package configmap

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
)

func TestNewFanoutConfig(t *testing.T) {
	testCases := []struct {
		name        string
		config      string
		expected    *multichannelfanout.Config
		expectedErr bool
	}{
		{
			name:        "no data",
			expectedErr: true,
		},
		{
			name: "invalid YAML",
			config: `
				key:
				  - value
				 - different indent level
				`,
			expectedErr: true,
		},
		{
			name:        "valid YAML -- invalid JSON",
			config:      "{ nil: Key }",
			expectedErr: true,
		},
		{
			name:        "unknown field",
			config:      "{ channelConfigs: [ { not: a-defined-field } ] }",
			expectedErr: true,
		},
		{
			name: "valid",
			config: `
				channelConfigs:
				  - namespace: default
					name: c1
					fanoutConfig:
					  subscriptions:
						- subscriberURI: event-changer.default.svc.cluster.local
						  replyURI: message-dumper-bar.default.svc.cluster.local
						- subscriberURI: message-dumper-foo.default.svc.cluster.local
						- replyURI: message-dumper-bar.default.svc.cluster.local
				  - namespace: default
					name: c2
					fanoutConfig:
					  subscriptions:
						- replyURI: message-dumper-foo.default.svc.cluster.local
				  - namespace: other
					name: c3
					fanoutConfig:
					  subscriptions:
						- replyURI: message-dumper-foo.default.svc.cluster.local
				`,
			expected: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SubscriberURI: "event-changer.default.svc.cluster.local",
									ReplyURI:      "message-dumper-bar.default.svc.cluster.local",
								},
								{
									SubscriberURI: "message-dumper-foo.default.svc.cluster.local",
								},
								{
									ReplyURI: "message-dumper-bar.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "c2",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "message-dumper-foo.default.svc.cluster.local",
								},
							},
						},
					},
					{
						Namespace: "other",
						Name:      "c3",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									ReplyURI: "message-dumper-foo.default.svc.cluster.local",
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data := formatData(tc.config)
			c, e := NewFanoutConfig(zap.NewNop(), data)
			if tc.expectedErr {
				if e == nil {
					t.Errorf("Expected an error, actual nil")
				}
				return
			}
			if !cmp.Equal(c, tc.expected) {
				t.Errorf("Unexpected config. Expected '%v'. Actual '%v'.", tc.expected, c)
			}
		})
	}
}

func TestSerializeConfig(t *testing.T) {
	testCases := map[string]struct {
		config *multichannelfanout.Config
	}{
		"empty config": {
			config: &multichannelfanout.Config{},
		},
		"full config": {
			config: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SubscriberURI: "foo.example.com",
									ReplyURI:      "bar.example.com",
								},
								{
									ReplyURI: "qux.example.com",
								},
								{
									SubscriberURI: "baz.example.com",
								},
								{},
							},
						},
					},
					{
						Namespace: "other",
						Name:      "no-subs",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{},
						},
					},
				},
			},
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			s, err := SerializeConfig(*tc.config)
			if err != nil {
				t.Errorf("Unexpected error serializing config: %v", err)
			}
			rt, err := NewFanoutConfig(zap.NewNop(), s)
			if err != nil {
				t.Errorf("Unexpected error deserializing: %v", err)
			}
			if diff := cmp.Diff(tc.config, rt); diff != "" {
				t.Errorf("Unexpected error roundtripping the config (-want, +got): %v", diff)
			}
		})
	}
}

func formatData(config string) map[string]string {
	data := make(map[string]string)
	if config != "" {
		// Golang editors tend to replace leading spaces with tabs. YAML is left whitespace
		// sensitive and disallows tabs, so let's replace the tabs with four spaces.
		leftSpaceConfig := strings.Replace(config, "\t", "    ", -1)
		data[MultiChannelFanoutConfigKey] = leftSpaceConfig
	}
	return data
}
