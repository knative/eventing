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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/pkg/apis"
)

var replaceDomain = apis.HTTP("replaceDomain")

const (
	hostName = "a.b.c.d"
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
	ctx := context.Background()
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	tctx.Method = http.MethodPost
	tctx.Host = hostName
	tctx.URI = "/"
	ctx = cehttp.WithTransportContext(ctx, tctx)

	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetType("testtype")
	event.SetSource("testsource")
	event.SetData("")

	resp := &cloudevents.EventResponse{}
	h.ServeHTTP(ctx, event, resp)
	if resp.Status != http.StatusAccepted {
		t.Errorf("Unexpected response code. Expected 202. Actual %v", resp.Status)
	}
}

type successHandler struct{}

func (*successHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = r.Body.Close()
}

func replaceDomains(c multichannelfanout.Config, replacement string) multichannelfanout.Config {
	for i, cc := range c.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.ReplyURI == replaceDomain {
				sub.ReplyURI = apis.HTTP(replacement)
			}
			if sub.SubscriberURI == replaceDomain {
				sub.SubscriberURI = apis.HTTP(replacement)
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		c.ChannelConfigs[i] = cc
	}
	return c
}
