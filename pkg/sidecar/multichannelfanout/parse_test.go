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

package multichannelfanout

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"go.uber.org/zap"
)

func TestConfigMapData(t *testing.T) {
	testCases := []struct {
		name        string
		config      string
		expected    *Config
		expectedErr bool
	}{
		{
			name:     "no data",
			expected: &Config{},
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
			expected: &Config{
				ChannelConfigs: []ChannelConfig{
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
			if tc.name != "valid" {
				return
			}
			c, e := Parse(zap.NewNop(), formatData(tc.config))
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

func formatData(config string) string {
	// Golang editors tend to replace leading spaces with tabs. YAML is left whitespace
	// sensitive and disallows tabs, so let's replace the tabs with four spaces.
	return strings.Replace(config, "\t", "    ", -1)
}
