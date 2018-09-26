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
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/sidecar/clientfactory/fake"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
			if key := MakeChannelKey(tc.namespace, tc.name); key != tc.key {
				t.Errorf("Unexpected ChannelKey. Expected '%v'. Actual '%v'", tc.key, key)
			}
		})
	}
}

func TestGetChannelKey(t *testing.T) {
	testCases := []struct {
		name        string
		headers     map[string]string
		expected    string
		expectedErr bool
	}{
		{
			name:        "no header",
			expectedErr: true,
		},
		{
			name: "header missing",
			headers: map[string]string{
				"User-Agent": "1234",
			},
			expectedErr: true,
		},
		{
			name: "header present without value",
			headers: map[string]string{
				ChannelKeyHeader: "",
			},
			expected: "",
		},
		{
			name: "header present",
			headers: map[string]string{
				ChannelKeyHeader: "default/c1",
			},
			expected: "default/c1",
		},
	}
	requestWithHeaders := func(headers map[string]string) *http.Request {
		r := httptest.NewRequest("POST", "http://target/", strings.NewReader("{}"))
		for existing := range r.Header {
			r.Header.Del(existing)
		}
		for h, v := range headers {
			r.Header.Set(h, v)
		}
		return r
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := requestWithHeaders(tc.headers)
			key, err := getChannelKey(r)
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected an error, did not get one.")
				}
			}
			if key != tc.expected {
				t.Errorf("Incorrect channel key. Expected '%v'. Actual '%v'", tc.expected, key)
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
			_, err := NewHandler(zap.NewNop(), tc.config, &fake.ClientFactory{})
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
					Subscriptions: []fanout.Subscription{
						{
							CallDomain: "calldomain",
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
					Subscriptions: []fanout.Subscription{
						{
							ToDomain: "todomain",
						},
					},
				},
			},
		},
	}
	if cmp.Equal(orig, updated) {
		t.Errorf("Orig and updated must be different")
	}
	h, err := NewHandler(zap.NewNop(), orig, &fake.ClientFactory{
		CreateErr: fmt.Errorf("random error that makes this client unique"),
	})
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
	if h.clientFactory != newH.clientFactory {
		t.Errorf("Did not copy clientFactory")
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
					Subscriptions: []fanout.Subscription{
						{
							CallDomain: "calldomain",
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
							Subscriptions: []fanout.Subscription{
								{
									CallDomain: "different",
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
			h, err := NewHandler(zap.NewNop(), tc.orig, &fake.ClientFactory{})
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
	testCases := []struct {
		name               string
		config             Config
		cf                 *fake.ClientFactory
		h                  http.Header
		key                string
		expectedStatusCode int
		requestDomain      string
	}{
		{
			name:               "non-existent channel",
			config:             Config{},
			cf:                 &fake.ClientFactory{},
			key:                "default/does-not-exist",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name:               "no channel key",
			config:             Config{},
			cf:                 &fake.ClientFactory{},
			key:                "",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "pass through failure",
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "first-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									ToDomain: "first-to-domain",
								},
							},
						},
					},
				},
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusInternalServerError,
						Body:       ioutil.NopCloser(strings.NewReader("{}")),
					},
				},
			},
			key:                "default/first-channel",
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "choose channel",
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "first-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									ToDomain: "first-to-domain",
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "second-channel",
						FanoutConfig: fanout.Config{
							Subscriptions: []fanout.Subscription{
								{
									CallDomain: "second-call-domain",
								},
							},
						},
					},
				},
			},
			cf: &fake.ClientFactory{
				Resp: []*http.Response{
					{
						StatusCode: http.StatusOK,
						Body:       ioutil.NopCloser(strings.NewReader("{}")),
					},
				},
			},
			key:                "default/second-channel",
			requestDomain:      "second-call-domain",
			expectedStatusCode: http.StatusOK,
		},
	}
	requestWithChannelKey := func(key string) *http.Request {
		r := httptest.NewRequest("POST", "http://target/", strings.NewReader("{}"))
		if key != "" {
			r.Header.Set(ChannelKeyHeader, key)
		}
		return r
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, err := NewHandler(zap.NewNop(), tc.config, tc.cf)
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
			if tc.requestDomain != "" {
				reqs := tc.cf.GetRequests()
				if reqs[0].Host != tc.requestDomain {
					t.Errorf("Called incorrect domain. Expected: '%v'. Actual '%v'", tc.requestDomain, reqs[0].Host)
				}
			}
		})
	}
}
