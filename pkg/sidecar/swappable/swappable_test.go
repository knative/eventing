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
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	namespace     = "default"
	name          = "channel1"
	replaceDomain = "replaceDomain"
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
							Namespace: namespace,
							Name:      name,
							FanoutConfig: fanout.Config{
								Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
									{
										CallableDomain: replaceDomain,
									},
								},
							},
						},
					},
				},
				{
					ChannelConfigs: []multichannelfanout.ChannelConfig{
						{
							Namespace: namespace,
							Name:      name,
							FanoutConfig: fanout.Config{
								Subscriptions: []duckv1alpha1.ChannelSubscriberSpec{
									{
										SinkableDomain: replaceDomain,
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

func updateConfigAndTest(t *testing.T, h *Handler, config multichannelfanout.Config) {
	server := httptest.NewServer(&successHandler{})
	defer server.Close()

	nh, err := multichannelfanout.NewHandler(zap.NewNop(), replaceDomains(config, server.URL[7:]))
	if err != nil {
		t.Errorf("Unexpected error creating multiChannelFanoutHandler: %v", err)
	}
	orig := h.getMultiChannelFanoutHandler()
	h.setMultiChannelFanoutHandler(nh)
	if orig == h.getMultiChannelFanoutHandler() {
		t.Errorf("Expected the inner multiChannelFanoutHandler to change, it didn't: %v", orig)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, makeRequest(namespace, name))
	if w.Code != http.StatusOK {
		t.Errorf("Unexpected response code. Expected 200. Actual %v", w.Code)
	}
}

type successHandler struct{}

func (*successHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	r.Body.Close()
}

func makeRequest(namespace, name string) *http.Request {
	r := httptest.NewRequest("POST", fmt.Sprintf("http://%s.%s/", name, namespace), strings.NewReader(""))
	return r
}

func replaceDomains(c multichannelfanout.Config, replacement string) multichannelfanout.Config {
	for i, cc := range c.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.SinkableDomain == replaceDomain {
				sub.SinkableDomain = replacement
			}
			if sub.CallableDomain == replaceDomain {
				sub.CallableDomain = replacement
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		c.ChannelConfigs[i] = cc
	}
	return c
}
