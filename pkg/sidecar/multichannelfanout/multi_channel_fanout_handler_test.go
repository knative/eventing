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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"go.uber.org/zap"
)

const (
	// The httptest.Server's host name will replace this value in all ChannelConfigs.
	replaceDomain = "replaceDomain"
)

func TestMakeChannelKey(t *testing.T) {
	testCases := []struct {
		namespace string
		name      string
		key       string
	}{
		{
			namespace: "default",
			name:      "channel",
			key:       "default/channel",
		},
		{
			namespace: "foo",
			name:      "bar",
			key:       "foo/bar",
		},
	}
	for _, tc := range testCases {
		name := fmt.Sprintf("%s, %s -> %s", tc.namespace, tc.name, tc.key)
		t.Run(name, func(t *testing.T) {
			if key := makeChannelKey(tc.namespace, tc.name); key != tc.key {
				t.Errorf("Unexpected ChannelKey. Expected '%v'. Actual '%v'", tc.key, key)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	testCases := []struct {
		name      string
		config    Config
		createErr string
	}{
		{
			name: "duplicate channel key",
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "duplicate",
					},
					{
						Namespace: "default",
						Name:      "duplicate",
					},
				},
			},
			createErr: "duplicate channel key: default/duplicate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewHandler(zap.NewNop(), tc.config)
			if tc.createErr != "" {
				if err == nil {
					t.Errorf("Expected NewHandler error: '%v'. Actual nil", tc.createErr)
				} else if err.Error() != tc.createErr {
					t.Errorf("Unexpected NewHandler error. Expected '%v'. Actual '%v'", tc.createErr, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected NewHandler error. Expected nil. Actual '%v'", err)
			}
		})
	}
}

func TestCopyWithNewConfig(t *testing.T) {
	orig := Config{
		ChannelConfigs: []ChannelConfig{
			{
				Namespace: "default",
				Name:      "c1",
				FanoutConfig: fanout.Config{
					Subscriptions: []eventingduck.ChannelSubscriberSpec{
						{
							CallableURI: "callabledomain",
						},
					},
				},
			},
		},
	}
	updated := Config{
		ChannelConfigs: []ChannelConfig{
			{
				Namespace: "default",
				Name:      "somethingdifferent",
				FanoutConfig: fanout.Config{
					Subscriptions: []eventingduck.ChannelSubscriberSpec{
						{
							SinkableURI: "sinkabledomain",
						},
					},
				},
			},
		},
	}
	if cmp.Equal(orig, updated) {
		t.Errorf("Orig and updated must be different")
	}
	h, err := NewHandler(zap.NewNop(), orig)
	if err != nil {
		t.Errorf("Unable to create handler, %v", err)
	}
	if !cmp.Equal(h.config, orig) {
		t.Errorf("Incorrect config. Expected '%v'. Actual '%v'", orig, h.config)
	}
	newH, err := h.CopyWithNewConfig(updated)
	if err != nil {
		t.Errorf("Unable to copy handler: %v", err)
	}
	if h.logger != newH.logger {
		t.Errorf("Did not copy logger")
	}
	if !cmp.Equal(newH.config, updated) {
		t.Errorf("Incorrect copied config. Expected '%v'. Actual '%v'", updated, newH.config)
	}
}

func TestConfigDiff(t *testing.T) {
	config := Config{
		ChannelConfigs: []ChannelConfig{
			{
				Namespace: "default",
				Name:      "c1",
				FanoutConfig: fanout.Config{
					Subscriptions: []eventingduck.ChannelSubscriberSpec{
						{
							CallableURI: "callabledomain",
						},
					},
				},
			},
		},
	}
	testCases := []struct {
		name         string
		orig         Config
		updated      Config
		expectedDiff bool
	}{
		{
			name:         "same",
			orig:         config,
			updated:      config,
			expectedDiff: false,
		},
		{
			name: "different",
			orig: config,
			updated: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "c1",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									CallableURI: "different",
								},
							},
						},
					},
				},
			},
			expectedDiff: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := NewHandler(zap.NewNop(), tc.orig)
			if err != nil {
				t.Errorf("Unable to create handler: %v", err)
			}
			diff := h.ConfigDiff(tc.updated)

			if hasDiff := diff != ""; hasDiff != tc.expectedDiff {
				t.Errorf("Unexpected diff result. Expected %v. Actual %v", tc.expectedDiff, hasDiff)
			}
		})
	}
}

func TestServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		name               string
		config             Config
		respStatusCode     int
		key                string
		expectedStatusCode int
	}{
		"non-existent channel": {
			config:             Config{},
			key:                "default.does-not-exist",
			expectedStatusCode: http.StatusInternalServerError,
		},
		"pass through failure": {
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "first-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SinkableURI: replaceDomain,
								},
							},
						},
					},
				},
			},
			respStatusCode:     http.StatusInternalServerError,
			key:                "first-channel.default",
			expectedStatusCode: http.StatusInternalServerError,
		},
		"choose channel": {
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "first-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									SinkableURI: "first-to-domain",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "second-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.ChannelSubscriberSpec{
								{
									CallableURI: replaceDomain,
								},
							},
						},
					},
				},
			},
			respStatusCode:     http.StatusOK,
			key:                "second-channel.default",
			expectedStatusCode: http.StatusAccepted,
		},
	}
	requestWithChannelKey := func(key string) *http.Request {
		r := httptest.NewRequest("POST", fmt.Sprintf("http://%s/", key), strings.NewReader("{}"))
		return r
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			server := httptest.NewServer(&fakeHandler{statusCode: tc.respStatusCode})
			defer server.Close()

			// Rewrite the replaceDomains to call the server we just created.
			replaceDomains(tc.config, server.URL[7:])

			h, err := NewHandler(zap.NewNop(), tc.config)
			if err != nil {
				t.Errorf("Unexpected NewHandler error: '%v'", err)
			}

			r := requestWithChannelKey(tc.key)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			resp := w.Result()
			if resp.StatusCode != tc.expectedStatusCode {
				t.Errorf("Unexpected status code. Expected %v, actual %v", tc.expectedStatusCode, resp.StatusCode)
			}
			if w.Body.String() != "" {
				t.Errorf("Expected empty response body. Actual: %v", w.Body)
			}
		})
	}
}

func replaceDomains(config Config, replacement string) {
	for i, cc := range config.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.CallableURI == replaceDomain {
				sub.CallableURI = replacement
			}
			if sub.SinkableURI == replaceDomain {
				sub.SinkableURI = replacement
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		config.ChannelConfigs[i] = cc
	}
}

type fakeHandler struct {
	statusCode int
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	w.WriteHeader(h.statusCode)
}
