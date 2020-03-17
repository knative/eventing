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

package swappable

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/binding"
	bindingshttp "github.com/cloudevents/sdk-go/pkg/protocol/http"
	"go.uber.org/zap"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
)

func TestMessageHandler(t *testing.T) {
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
			h, err := NewEmptyMessageHandler(context.TODO(), zap.NewNop())
			if err != nil {
				t.Errorf("Unexpected error creating handler: %v", err)
			}
			for _, c := range tc.configs {
				updateConfigAndTestBinding(t, h, c)
			}
		})
	}
}

func TestMessageHandler_InvalidConfigChange(t *testing.T) {
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

func TestMessageHandler_NilConfigChange(t *testing.T) {
	h, err := NewEmptyMessageHandler(context.TODO(), zap.NewNop())
	if err != nil {
		t.Errorf("Unexpected error creating handler: %v", err)
	}

	err = h.UpdateConfig(context.TODO(), nil)
	if err == nil {
		t.Errorf("Expected an error when updating to a nil config.")
	}
}

func updateConfigAndTestBinding(t *testing.T, h *MessageHandler, config multichannelfanout.Config) {
	server := httptest.NewServer(&successHandler{})
	defer server.Close()

	rc := replaceDomains(config, server.URL[7:])
	orig := h.fanout.Load()
	err := h.UpdateConfig(context.TODO(), &rc)
	if err != nil {
		t.Errorf("Unexpected error updating config: %+v", err)
	}
	if orig == h.fanout.Load() {
		t.Errorf("Expected the inner multiChannelFanoutHandler to change, it didn't: %v", orig)
	}

	assertRequestBindingAccepted(t, h)
}

func assertRequestBindingAccepted(t *testing.T, h *MessageHandler) {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("testtype")
	event.SetSource("testsource")
	event.SetData("text/plain", "")

	req := httptest.NewRequest(http.MethodPost, "http://"+hostName+"/", nil)
	err := bindingshttp.WriteRequest(context.Background(), binding.ToMessage(&event), req, binding.TransformerFactories{})
	if err != nil {
		t.Fatal(err)
	}

	resp := httptest.ResponseRecorder{}

	h.ServeHTTP(&resp, req)
	if resp.Code != http.StatusAccepted {
		t.Errorf("Unexpected response code. Expected 202. Actual %v", resp.Code)
	}
}
