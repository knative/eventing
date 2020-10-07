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

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	bindingshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
)

var replaceDomain = apis.HTTP("replaceDomain").URL()

const (
	hostName = "a.b.c.d"
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
								Subscriptions: []fanout.Subscription{
									{
										Subscriber: replaceDomain,
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
								Subscriptions: []fanout.Subscription{
									{
										Reply: replaceDomain,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
			h, err := NewEmptyMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)
			if err != nil {
				t.Error("Unexpected error creating handler:", err)
			}
			for _, c := range tc.configs {
				updateConfigAndTest(t, h, c)
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
							Subscriptions: []fanout.Subscription{
								{
									Subscriber: replaceDomain,
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
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
			h, err := NewEmptyMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)
			if err != nil {
				t.Error("Unexpected error creating handler:", err)
			}

			server := httptest.NewServer(http.HandlerFunc(successHandler))
			defer server.Close()

			rc := replaceDomains(tc.initialConfig, server.URL[7:])
			err = h.UpdateConfig(context.TODO(), channel.EventDispatcherConfig{}, &rc)
			if err != nil {
				t.Error("Unexpected error updating to initial config:", tc.initialConfig)
			}
			assertRequestAccepted(t, h)

			// Try to update to the new config, it will fail. But we should still be able to hit the
			// original server.
			ruc := replaceDomains(tc.badUpdateConfig, server.URL[7:])
			err = h.UpdateConfig(context.TODO(), channel.EventDispatcherConfig{}, &ruc)
			if err == nil {
				t.Errorf("Expected an error when updating to a bad config.")
			}
			assertRequestAccepted(t, h)
		})
	}
}

func TestMessageHandler_NilConfigChange(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	h, err := NewEmptyMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)
	if err != nil {
		t.Error("Unexpected error creating handler:", err)
	}

	err = h.UpdateConfig(context.TODO(), channel.EventDispatcherConfig{}, nil)
	if err == nil {
		t.Errorf("Expected an error when updating to a nil config.")
	}
}

func updateConfigAndTest(t *testing.T, h *MessageHandler, config multichannelfanout.Config) {
	server := httptest.NewServer(http.HandlerFunc(successHandler))
	defer server.Close()

	rc := replaceDomains(config, server.URL[7:])
	orig := h.fanout.Load()
	err := h.UpdateConfig(context.TODO(), channel.EventDispatcherConfig{}, &rc)
	if err != nil {
		t.Errorf("Unexpected error updating config: %+v", err)
	}
	if orig == h.fanout.Load() {
		t.Error("Expected the inner multiChannelFanoutHandler to change, it didn't:", orig)
	}

	assertRequestAccepted(t, h)
}

func assertRequestAccepted(t *testing.T, h *MessageHandler) {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("testtype")
	event.SetSource("testsource")
	err := event.SetData("text/plain", "")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://"+hostName+"/", nil)
	err = bindingshttp.WriteRequest(context.Background(), binding.ToMessage(&event), req)
	if err != nil {
		t.Fatal(err)
	}

	resp := httptest.ResponseRecorder{}

	h.ServeHTTP(&resp, req)
	if resp.Code != http.StatusAccepted {
		t.Error("Unexpected response code. Expected 202. Actual", resp.Code)
	}
}

func successHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = r.Body.Close()
}

func replaceDomains(c multichannelfanout.Config, replacement string) multichannelfanout.Config {
	for i, cc := range c.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.Subscriber == replaceDomain {
				sub.Subscriber = apis.HTTP(replacement).URL()
			}
			if sub.Reply == replaceDomain {
				sub.Reply = apis.HTTP(replacement).URL()
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		c.ChannelConfigs[i] = cc
	}
	return c
}
