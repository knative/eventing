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

package fanout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	bindingshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/channel"
)

func TestFanoutMessageHandler_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		receiverFunc   channel.MessageReceiverFunc
		timeout        time.Duration
		subs           []eventingduck.SubscriberSpec
		subscriber     func(http.ResponseWriter, *http.Request)
		channel        func(http.ResponseWriter, *http.Request)
		expectedStatus int
		asyncHandler   bool
		skip           string
	}{
		"rejected by receiver": {
			receiverFunc: func(context.Context, channel.ChannelReference, binding.Message, http.Header) error {
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
				time.Sleep(time.Second)
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

			h, err := NewMessageHandler(context.TODO(), zap.NewNop(), Config{Subscriptions: subs})
			if err != nil {
				t.Fatalf("NewHandler failed. Error:%s", err)
			}
			if tc.asyncHandler {
				h.config.AsyncHandler = true
			}
			if tc.receiverFunc != nil {
				receiver, err := channel.NewMessageReceiver(context.TODO(), tc.receiverFunc, zap.NewNop())
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

			event := makeCloudEventNew()
			req := httptest.NewRequest(http.MethodPost, "http://channelname.channelnamespace/", nil)

			ctx := context.Background()
			err = bindingshttp.WriteRequest(ctx, binding.ToMessage(&event), req, binding.TransformerFactories{})
			if err != nil {
				t.Fatal(err)
			}

			resp := httptest.ResponseRecorder{}

			h.ServeHTTP(&resp, req)
			if resp.Code != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Code)
			}
		})
	}
}

func makeCloudEventNew() cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("com.example.someevent")
	event.SetSource("/mycontext")
	event.SetID("A234-1234-1234")
	event.SetExtension("comexampleextension", "value")
	event.SetData(cloudevents.ApplicationXML, "<much wow=\"xml\"/>")
	return event
}
