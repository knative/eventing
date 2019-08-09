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

package swappable

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/provisioners/fanout"
	"knative.dev/eventing/pkg/provisioners/multichannelfanout"
)

const (
	replaceDomain = "replaceDomain"
	hostName      = "a.b.c.d"
)

func TestHandler(t *testing.T) {
	testCases := map[string]struct {
		configs []multichannelfanout.Config
	}{
		"config change": {
			configs: []multichannelfanout.Config{
				{
					ChannelConfigs: []multichannelfanout.ChannelConfig{
						{
							HostName: hostName,
							FanoutConfig: fanout.Config{
								Subscriptions: []eventingduck.SubscriberSpec{
									{
										SubscriberURI: replaceDomain,
									},
								},
							},
						},
					},
				},
				{
					ChannelConfigs: []multichannelfanout.ChannelConfig{
						{
							HostName: hostName,
							FanoutConfig: fanout.Config{
								Subscriptions: []eventingduck.SubscriberSpec{
									{
										ReplyURI: replaceDomain,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h, err := NewEmptyHandler(zap.NewNop())
			if err != nil {
				t.Errorf("Unexpected error creating handler: %v", err)
			}
			for _, c := range tc.configs {
				updateConfigAndTest(t, h, c)
			}
		})
	}
}

func TestHandler_InvalidConfigChange(t *testing.T) {
	testCases := map[string]struct {
		initialConfig   multichannelfanout.Config
		badUpdateConfig multichannelfanout.Config
	}{
		"invalid config change": {
			initialConfig: multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						HostName: hostName,
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									SubscriberURI: replaceDomain,
								},
							},
						},
					},
				},
			},
			badUpdateConfig: multichannelfanout.Config{
				// Duplicate (namespace, name).
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						HostName: hostName,
					},
					{
						HostName: hostName,
					},
				},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h, err := NewEmptyHandler(zap.NewNop())
			if err != nil {
				t.Errorf("Unexpected error creating handler: %v", err)
			}

			server := httptest.NewServer(&successHandler{})
			defer server.Close()

			rc := replaceDomains(tc.initialConfig, server.URL[7:])
			err = h.UpdateConfig(&rc)
			if err != nil {
				t.Errorf("Unexpected error updating to initial config: %v", tc.initialConfig)
			}
			assertRequestAccepted(t, h)

			// Try to update to the new config, it will fail. But we should still be able to hit the
			// original server.
			ruc := replaceDomains(tc.badUpdateConfig, server.URL[7:])
			err = h.UpdateConfig(&ruc)
			if err == nil {
				t.Errorf("Expected an error when updating to a bad config.")
			}
			assertRequestAccepted(t, h)
		})
	}
}

func TestHandler_NilConfigChange(t *testing.T) {
	h, err := NewEmptyHandler(zap.NewNop())
	if err != nil {
		t.Errorf("Unexpected error creating handler: %v", err)
	}

	err = h.UpdateConfig(nil)
	if err == nil {
		t.Errorf("Expected an error when updating to a nil config.")
	}
}

func updateConfigAndTest(t *testing.T, h *Handler, config multichannelfanout.Config) {
	server := httptest.NewServer(&successHandler{})
	defer server.Close()

	rc := replaceDomains(config, server.URL[7:])
	orig := h.fanout.Load()
	err := h.UpdateConfig(&rc)
	if err != nil {
		t.Errorf("Unexpected error updating config: %+v", err)
	}
	if orig == h.fanout.Load() {
		t.Errorf("Expected the inner multiChannelFanoutHandler to change, it didn't: %v", orig)
	}

	assertRequestAccepted(t, h)
}

func assertRequestAccepted(t *testing.T, h *Handler) {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeRequest(hostName))
	if w.Code != http.StatusAccepted {
		t.Errorf("Unexpected response code. Expected 202. Actual %v", w.Code)
	}
}

type successHandler struct{}

func (*successHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = r.Body.Close()
}

func makeRequest(hostName string) *http.Request {
	r := httptest.NewRequest("POST", fmt.Sprintf("http://%s/", hostName), strings.NewReader(""))
	return r
}

func replaceDomains(c multichannelfanout.Config, replacement string) multichannelfanout.Config {
	for i, cc := range c.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.ReplyURI == replaceDomain {
				sub.ReplyURI = replacement
			}
			if sub.SubscriberURI == replaceDomain {
				sub.SubscriberURI = replacement
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		c.ChannelConfigs[i] = cc
	}
	return c
}
