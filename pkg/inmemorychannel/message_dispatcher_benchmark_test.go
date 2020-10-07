/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package inmemorychannel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	protocolhttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/channel/swappable"
	"knative.dev/eventing/pkg/kncloudevents"
)

// This test emulates a real dispatcher usage
// send -> channela -> sub aaaa -> transformationsServer -> channelb -> sub bbbb -> receiver
func BenchmarkDispatcher_dispatch_ok_through_2_channels(b *testing.B) {
	logger := zap.NewNop()
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
	sh, err := swappable.NewEmptyMessageHandler(context.TODO(), logger, nil, reporter)
	if err != nil {
		b.Fatal(err)
	}

	dispatcherArgs := &InMemoryMessageDispatcherArgs{
		Port:         8080,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
		Handler:      sh,
		Logger:       logger,
	}

	dispatcher := NewMessageDispatcher(dispatcherArgs)
	requestHandler := kncloudevents.CreateHandler(dispatcher.handler)

	channelAUrl := mustParseUrl(b, "http://channela.svc/")
	transformationsUrl := mustParseUrl(b, "http://transformations.svc/")
	channelBUrl := mustParseUrl(b, "http://channelb.svc/")
	receiverUrl := mustParseUrl(b, "http://receiver.svc/")

	// The message flow is:
	// send -> channela -> sub aaaa -> transformationsServer -> channelb -> sub bbbb -> receiver
	config := multichannelfanout.Config{
		ChannelConfigs: []multichannelfanout.ChannelConfig{
			{
				Namespace: "default",
				Name:      "channela",
				HostName:  "channela.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []fanout.Subscription{{
						Subscriber: transformationsUrl.URL(),
						Reply:      channelBUrl.URL(),
					}},
				},
			},
			{
				Namespace: "default",
				Name:      "channelb",
				HostName:  "channelb.svc",
				FanoutConfig: fanout.Config{
					AsyncHandler: false,
					Subscriptions: []fanout.Subscription{{
						Subscriber: receiverUrl.URL(),
					}},
				},
			},
		},
	}

	// Let's mock this stuff!
	httpSender, err := kncloudevents.NewHTTPMessageSender(nil, channelAUrl.String())
	if err != nil {
		b.Fatal(err)
	}
	httpSender.Client = mockedHTTPClient(clientMock(channelAUrl.Host, transformationsUrl.Host, channelBUrl.Host, receiverUrl.Host, requestHandler))

	multiChannelFanoutHandler, err := multichannelfanout.NewMessageHandler(context.TODO(), logger, channel.NewMessageDispatcherFromSender(logger, httpSender), config, reporter)
	if err != nil {
		b.Fatal(err)
	}

	sh.SetHandler(multiChannelFanoutHandler)

	// Start the bench
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := httpSender.NewCloudEventRequest(context.Background())

		event := test.FullEvent()
		_ = protocolhttp.WriteRequest(context.Background(), binding.ToMessage(&event), req)

		_, _ = httpSender.Send(req)
	}
}

func clientMock(channelAHost string, transformationsHost string, channelBHost string, receiverHost string, channelHandler http.Handler) roundTripFunc {
	return func(req *http.Request) *http.Response {
		response := httptest.ResponseRecorder{}
		if req.URL.Host == channelAHost || req.URL.Host == channelBHost {
			channelHandler.ServeHTTP(&response, req)
			return response.Result()
		}
		if req.URL.Host == transformationsHost {
			message := protocolhttp.NewMessageFromHttpRequest(req)
			defer message.Finish(nil)

			_ = protocolhttp.WriteResponseWriter(
				context.Background(),
				message,
				200,
				&response,
				transformer.AddExtension("transformed", "true"),
			)
			return response.Result()
		}
		if req.URL.Host == receiverHost {
			transformed := req.Header.Get("ce-transformed")

			if transformed != "true" {
				response.WriteHeader(500)
			} else {
				response.WriteHeader(200)
			}

			return response.Result()
		}

		response.WriteHeader(404)
		return response.Result()
	}
}

type roundTripFunc func(req *http.Request) *http.Response

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func mockedHTTPClient(fn roundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}
