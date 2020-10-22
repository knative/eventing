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

package multichannelfanout

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
)

var (
	// The httptest.Server's host name will replace this value in all ChannelConfigs.
	replaceDomain = apis.HTTP("replaceDomain").URL()
)

func TestNewMessageHandlerWithConfig(t *testing.T) {
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
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
			_, err := NewMessageHandlerWithConfig(
				context.TODO(),
				logger,
				channel.NewMessageDispatcher(logger),
				tc.config,
				reporter,
			)
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

func TestNewMessageHandler(t *testing.T) {
	handlerName := "handler.example.com"
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))

	handler := NewMessageHandler(context.TODO(), logger, channel.NewMessageDispatcher(logger), reporter)
	h := handler.GetChannelHandler(handlerName)
	if len(handler.handlers) != 0 {
		t.Errorf("non-empty handler map on creation")
	}
	if h != nil {
		t.Errorf("Found handler for %q but not expected", handlerName)
	}
	f, err := fanout.NewFanoutMessageHandler(logger, channel.NewMessageDispatcher(logger), fanout.Config{}, reporter)
	if err != nil {
		t.Error("Failed to create FanoutMessagHandler: ", err)
	}
	handler.SetChannelHandler(handlerName, f)
	h = handler.GetChannelHandler(handlerName)
	if h == nil {
		t.Error("Did not find handler")
	}
	handler.DeleteChannelHandler(handlerName)
	h = handler.GetChannelHandler(handlerName)
	if h != nil {
		t.Error("Found handler, but not supposed to be there after deleting")
	}

}

func TestServeHTTPMessageHandler(t *testing.T) {
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
							Subscriptions: []fanout.Subscription{
								{
									Reply: replaceDomain,
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
							Subscriptions: []fanout.Subscription{
								{
									Reply: apis.HTTP("first-to-domain").URL(),
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "second-channel",
						HostName:  "second-channel.default",
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
			respStatusCode:     http.StatusOK,
			key:                "second-channel.default",
			expectedStatusCode: http.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			server := httptest.NewServer(fakeHandler(tc.respStatusCode))
			defer server.Close()

			// Rewrite the replaceDomains to call the server we just created.
			replaceDomains(tc.config, server.URL[7:])

			logger := zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller()))
			reporter := channel.NewStatsReporter("testcontainer", "testpod")
			h, err := NewMessageHandlerWithConfig(context.TODO(), logger, channel.NewMessageDispatcher(logger), tc.config, reporter)
			if err != nil {
				t.Fatalf("Unexpected NewHandler error: '%v'", err)
			}

			ctx := context.Background()

			event := cloudevents.NewEvent(cloudevents.VersionV1)
			event.SetType("testtype")
			event.SetSource("testsource")
			event.SetData(cloudevents.ApplicationJSON, "{}")

			req := httptest.NewRequest(http.MethodPost, "http://"+tc.key+"/", nil)
			err = bindingshttp.WriteRequest(ctx, binding.ToMessage(&event), req)
			if err != nil {
				t.Fatal(err)
			}

			responseRecorder := httptest.ResponseRecorder{}

			h.ServeHTTP(&responseRecorder, req)
			response := responseRecorder.Result()
			if response.StatusCode != tc.expectedStatusCode {
				t.Errorf("Unexpected status code. Expected %v, actual %v", tc.expectedStatusCode, response.StatusCode)
			}

			var message binding.Message
			if response.Body != nil {
				message = bindingshttp.NewMessage(response.Header, response.Body)
			} else {
				message = bindingshttp.NewMessage(response.Header, nil)
			}
			if message.ReadEncoding() != binding.EncodingUnknown {
				t.Error("Expected EncodingUnknown. Actual:", message.ReadEncoding())
			}
		})
	}
}

func fakeHandler(statusCode int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		_ = r.Body.Close()
	}
}

func replaceDomains(config Config, replacement string) {
	for i, cc := range config.ChannelConfigs {
		for j, sub := range cc.FanoutConfig.Subscriptions {
			if sub.Subscriber == replaceDomain {
				sub.Subscriber = apis.HTTP(replacement).URL()
			}
			if sub.Reply == replaceDomain {
				sub.Reply = apis.HTTP(replacement).URL()
			}
			cc.FanoutConfig.Subscriptions[j] = sub
		}
		config.ChannelConfigs[i] = cc
	}
}
