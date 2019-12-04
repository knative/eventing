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

package fanout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel"
	"knative.dev/pkg/apis"
)

// Domains used in subscriptions, which will be replaced by the real domains of the started HTTP
// servers.
var (
	replaceSubscriber = apis.HTTP("replaceSubscriber")
	replaceChannel    = apis.HTTP("replaceChannel")
)

func makeCloudEvent() cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetType("com.example.someevent")
	event.SetSource("/mycontext")
	event.SetID("A234-1234-1234")
	event.SetExtension("comExampleExtension", "value")
	event.SetData("<much wow=\"xml\"/>")
	return event
}

func TestFanoutHandler_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		receiverFunc   channel.ReceiverFunc
		timeout        time.Duration
		subs           []eventingduck.SubscriberSpec
		subscriber     func(http.ResponseWriter, *http.Request)
		channel        func(http.ResponseWriter, *http.Request)
		expectedStatus int
		asyncHandler   bool
		skip           string
	}{
		"rejected by receiver": {
			receiverFunc: func(context.Context, channel.ChannelReference, cloudevents.Event) error {
				return errors.New("rejected by test-receiver")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		"fanout times out": {
			timeout: time.Millisecond,
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
				},
			},
			subscriber: func(writer http.ResponseWriter, _ *http.Request) {
				time.Sleep(10 * time.Millisecond)
				writer.WriteHeader(http.StatusAccepted)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		"zero subs succeed": {
			subs:           []eventingduck.SubscriberSpec{},
			expectedStatus: http.StatusAccepted,
		},
		"empty sub succeeds": {
			subs: []eventingduck.SubscriberSpec{
				{},
			},
			expectedStatus: http.StatusAccepted,
		},
		"reply fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					ReplyURI: replaceChannel,
				},
			},
			channel: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		"subscriber fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
				},
			},
			subscriber: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		"subscriber succeeds, result fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
			},
			subscriber: callableSucceed,
			channel: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusForbidden)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		"one sub succeeds": {
			skip: "FLAKE This test is flaky, server throws 500.",
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
			},
			subscriber: callableSucceed,
			channel: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			},
			expectedStatus: http.StatusAccepted,
		},
		"one sub succeeds, one sub fails": {
			skip: "RACE condition due to bug in cloudevents-sdk. Unskip it once the issue https://github.com/cloudevents/sdk-go/issues/193 is fixed",
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
			},
			subscriber:     callableSucceed,
			channel:        (&succeedOnce{}).handler,
			expectedStatus: http.StatusInternalServerError,
		},
		"all subs succeed": {
			skip: "FLAKE This test is flaky on constrained nodes.",
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
			},
			subscriber: callableSucceed,
			channel: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			},
			expectedStatus: http.StatusAccepted,
		},
		"all subs succeed with async handler": {
			skip: "RACE condition due to bug in cloudevents-sdk. Unskip it once the issue https://github.com/cloudevents/sdk-go/issues/193 is fixed",
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceChannel,
				},
			},
			subscriber: callableSucceed,
			channel: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			},
			expectedStatus: http.StatusAccepted,
			asyncHandler:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			if tc.skip != "" {
				t.Skip(tc.skip)
			}
			callableServer := httptest.NewServer(&fakeHandler{
				handler: tc.subscriber,
			})
			defer callableServer.Close()
			channelServer := httptest.NewServer(&fakeHandler{
				handler: tc.channel,
			})
			defer channelServer.Close()

			// Rewrite the subs to use the servers we just started.
			subs := make([]eventingduck.SubscriberSpec, 0)
			for _, sub := range tc.subs {
				if sub.SubscriberURI == replaceSubscriber {
					sub.SubscriberURI = apis.HTTP(callableServer.URL[7:]) // strip the leading 'http://'
				}
				if sub.ReplyURI == replaceChannel {
					sub.ReplyURI = apis.HTTP(channelServer.URL[7:]) // strip the leading 'http://'
				}
				subs = append(subs, sub)
			}

			h, err := NewHandler(zap.NewNop(), Config{Subscriptions: subs})
			if err != nil {
				t.Fatalf("NewHandler failed. Error:%s", err)
			}
			if tc.asyncHandler {
				h.config.AsyncHandler = true
			}
			if tc.receiverFunc != nil {
				receiver, err := channel.NewEventReceiver(tc.receiverFunc, zap.NewNop())
				if err != nil {
					t.Fatalf("NewEventReceiver failed. Error:%s", err)
				}
				h.receiver = receiver
			}
			if tc.timeout != 0 {
				h.timeout = tc.timeout
			} else {
				// Reasonable timeout for the tests.
				h.timeout = 100 * time.Millisecond
			}

			ctx := context.Background()
			tctx := cloudevents.HTTPTransportContextFrom(ctx)
			tctx.Method = http.MethodPost
			tctx.Host = "channelname.channelnamespace"
			tctx.URI = "/"
			ctx = cehttp.WithTransportContext(ctx, tctx)

			event := makeCloudEvent()
			resp := &cloudevents.EventResponse{}
			h.ServeHTTP(ctx, event, resp)
			if resp.Status != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Status)
			}
		})
	}
}

type fakeHandler struct {
	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.Body.Close()
	h.handler(w, r)
}

type succeedOnce struct {
	called atomic.Bool
}

func (s *succeedOnce) handler(w http.ResponseWriter, _ *http.Request) {
	if s.called.CAS(false, true) {
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusForbidden)
	}
}

func callableSucceed(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("ce-specversion", cloudevents.VersionV03)
	writer.Header().Set("ce-type", "com.example.someotherevent")
	writer.Header().Set("ce-source", "/myothercontext")
	writer.Header().Set("ce-id", "B234-1234-1234")
	writer.Header().Set("Content-Type", cloudevents.ApplicationJSON)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("{}"))
}
