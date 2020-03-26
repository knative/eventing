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
	"sync"
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
		receiverFunc        channel.UnbufferedMessageReceiverFunc
		timeout             time.Duration
		subs                []eventingduck.SubscriberSpec
		subscriber          func(http.ResponseWriter, *http.Request)
		subscriberReqs      int
		replier             func(http.ResponseWriter, *http.Request)
		replierReqs         int
		expectedStatus      int
		asyncExpectedStatus int
	}{
		"rejected by receiver": {
			receiverFunc: func(context.Context, channel.ChannelReference, binding.Message, []binding.TransformerFactory, http.Header) error {
				return errors.New("rejected by test-receiver")
			},
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusInternalServerError,
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
			subscriberReqs:      1,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"zero subs succeed": {
			subs:                []eventingduck.SubscriberSpec{},
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"empty sub succeeds": {
			subs: []eventingduck.SubscriberSpec{
				{},
			},
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"reply fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					ReplyURI: replaceReplier,
				},
			},
			replier: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNotFound)
			},
			replierReqs:         1,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
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
			subscriberReqs:      1,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"subscriber succeeds, result fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
			},
			subscriber: callableSucceed,
			replier: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusForbidden)
			},
			subscriberReqs:      1,
			replierReqs:         1,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"one sub succeeds": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
			},
			subscriber: callableSucceed,
			replier: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			},
			subscriberReqs:      1,
			replierReqs:         1,
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"one sub succeeds, one sub fails": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
			},
			subscriber:          callableSucceed,
			replier:             (&succeedOnce{}).handler,
			subscriberReqs:      2,
			replierReqs:         2,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"all subs succeed": {
			subs: []eventingduck.SubscriberSpec{
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
				{
					SubscriberURI: replaceSubscriber,
					ReplyURI:      replaceReplier,
				},
			},
			subscriber: callableSucceed,
			replier: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusAccepted)
			},
			subscriberReqs:      3,
			replierReqs:         3,
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run("sync - "+n, func(t *testing.T) {
			testFanoutMessageHandler(t, false, tc.receiverFunc, tc.timeout, tc.subs, tc.subscriber, tc.subscriberReqs, tc.replier, tc.replierReqs, tc.expectedStatus)
		})
		t.Run("async - "+n, func(t *testing.T) {
			testFanoutMessageHandler(t, true, tc.receiverFunc, tc.timeout, tc.subs, tc.subscriber, tc.subscriberReqs, tc.replier, tc.replierReqs, tc.asyncExpectedStatus)
		})
	}
}

func testFanoutMessageHandler(t *testing.T, async bool, receiverFunc channel.UnbufferedMessageReceiverFunc, timeout time.Duration, inSubs []eventingduck.SubscriberSpec, subscriberHandler func(http.ResponseWriter, *http.Request), subscriberReqs int, replierHandler func(http.ResponseWriter, *http.Request), replierReqs int, expectedStatus int) {
	var subscriberServerWg *sync.WaitGroup
	if subscriberReqs != 0 {
		subscriberServerWg = &sync.WaitGroup{}
		subscriberServerWg.Add(subscriberReqs)
	}
	subscriberServer := httptest.NewServer(&fakeHandlerWithWg{
		wg:      subscriberServerWg,
		handler: subscriberHandler,
	})
	defer subscriberServer.Close()

	var replierServerWg *sync.WaitGroup
	if replierReqs != 0 {
		replierServerWg = &sync.WaitGroup{}
		replierServerWg.Add(replierReqs)
	}
	replyServer := httptest.NewServer(&fakeHandlerWithWg{
		wg:      replierServerWg,
		handler: replierHandler,
	})
	defer replyServer.Close()

	// Rewrite the subs to use the servers we just started.
	subs := make([]eventingduck.SubscriberSpec, 0)
	for _, sub := range inSubs {
		if sub.SubscriberURI == replaceSubscriber {
			sub.SubscriberURI = apis.HTTP(subscriberServer.URL[7:]) // strip the leading 'http://'
		}
		if sub.ReplyURI == replaceReplier {
			sub.ReplyURI = apis.HTTP(replyServer.URL[7:]) // strip the leading 'http://'
		}
		subs = append(subs, sub)
	}

	h, err := NewMessageHandler(zap.NewNop(), Config{
		Subscriptions: subs,
		AsyncHandler:  async,
	})
	if err != nil {
		t.Fatalf("NewHandler failed. Error:%s", err)
	}

	if receiverFunc != nil {
		receiver, err := channel.NewMessageReceiver(receiverFunc, zap.NewNop())
		if err != nil {
			t.Fatalf("NewEventReceiver failed. Error:%s", err)
		}
		h.receiver = receiver
	}
	if timeout != 0 {
		h.timeout = timeout
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
	if resp.Code != expectedStatus {
		t.Errorf("Unexpected status code. Expected %v, Actual %v", expectedStatus, resp.Code)
	}

	if subscriberServerWg != nil {
		subscriberServerWg.Wait()
	}
	if replierServerWg != nil {
		replierServerWg.Wait()
	}
}

type fakeHandlerWithWg struct {
	wg      *sync.WaitGroup
	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandlerWithWg) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.Body.Close()
	h.handler(w, r)
	if h.wg != nil {
		h.wg.Done()
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
