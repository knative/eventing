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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/pkg/apis"
)

var (
	// The httptest.Server's host name will replace this value in all ChannelConfigs.
	replaceDomain = apis.HTTP("replaceDomain")
)

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
						HostName: "duplicatekey",
					},
					{
						HostName: "duplicatekey",
					},
				},
			},
			createErr: "duplicate channel key: duplicatekey",
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
					Subscriptions: []eventingduck.SubscriberSpec{
						{
							SubscriberURI: apis.HTTP("subscriberdomain"),
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
					Subscriptions: []eventingduck.SubscriberSpec{
						{
							ReplyURI: apis.HTTP("replydomain"),
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
					Subscriptions: []eventingduck.SubscriberSpec{
						{
							SubscriberURI: apis.HTTP("subscriberdomain"),
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
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									SubscriberURI: apis.HTTP("different"),
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
		"bad host": {
			config:             Config{},
			key:                "no-dot",
			expectedStatusCode: http.StatusInternalServerError,
		},
		"pass through failure": {
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "ns",
						Name:      "name",
						HostName:  "first-channel.default",
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
			respStatusCode:     http.StatusInternalServerError,
			key:                "first-channel.default",
			expectedStatusCode: http.StatusInternalServerError,
		},
		"choose channel": {
			config: Config{
				ChannelConfigs: []ChannelConfig{
					{

						Namespace: "ns",
						Name:      "name",
						HostName:  "first-channel.default",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									ReplyURI: apis.HTTP("first-to-domain"),
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "second-channel",
						HostName:  "second-channel.default",
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
			respStatusCode:     http.StatusOK,
			key:                "second-channel.default",
			expectedStatusCode: http.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			server := httptest.NewServer(&fakeHandler{statusCode: tc.respStatusCode})
			defer server.Close()

			// Rewrite the replaceDomains to call the server we just created.
			replaceDomains(tc.config, server.URL[7:])

			h, err := NewHandler(zap.NewNop(), tc.config)
			if err != nil {
				t.Fatalf("Unexpected NewHandler error: '%v'", err)
			}

			ctx := context.Background()
			tctx := cloudevents.HTTPTransportContextFrom(ctx)
			tctx.Method = http.MethodPost
			tctx.Host = tc.key
			tctx.URI = "/"
			ctx = cehttp.WithTransportContext(ctx, tctx)

			event := cloudevents.NewEvent(cloudevents.VersionV03)
			event.SetType("testtype")
			event.SetSource("testsource")
			event.SetData("{}")

			resp := &cloudevents.EventResponse{}
			h.ServeHTTP(ctx, event, resp)
			if resp.Status != tc.expectedStatusCode {
				t.Errorf("Unexpected status code. Expected %v, actual %v", tc.expectedStatusCode, resp.Status)
			}
			if resp.Event != nil {
				t.Errorf("Expected nil event. Actual: %s", resp.Event.String())
			}
		})
	}
}

func replaceDomains(config Config, replacement string) {
	for i, cc := range config.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.SubscriberURI == replaceDomain {
				sub.SubscriberURI = apis.HTTP(replacement)
			}
			if sub.ReplyURI == replaceDomain {
				sub.ReplyURI = apis.HTTP(replacement)
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
	w.WriteHeader(h.statusCode)
	_ = r.Body.Close()
}
