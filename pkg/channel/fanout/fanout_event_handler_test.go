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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/kncloudevents"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	bindingshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"go.opencensus.io/trace"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"

	"knative.dev/eventing/pkg/channel"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/system/testing"
)

// Domains used in subscriptions, which will be replaced by the real domains of the started HTTP
// servers.
var (
	replaceSubscriber = duckv1.Addressable{
		URL: apis.HTTP("replaceSubscriber"),
	}
	replaceReplier = duckv1.Addressable{
		URL: apis.HTTP("replaceReplier"),
	}
)

func TestSubscriberSpecToFanoutConfig(t *testing.T) {
	subscriberCACerts := "subscriber-certs"
	replyCACerts := "reply-certs"
	dlsCACerts := "dls-certs"
	linear := eventingduckv1.BackoffPolicyLinear
	delay := "PT1S"
	spec := &eventingduckv1.SubscriberSpec{
		SubscriberURI:     apis.HTTP("subscriber.example.com"),
		SubscriberCACerts: &subscriberCACerts,
		ReplyURI:          apis.HTTP("reply.example.com"),
		ReplyCACerts:      &replyCACerts,
		Delivery: &eventingduckv1.DeliverySpec{
			DeadLetterSink: &duckv1.Destination{
				Ref: &duckv1.KReference{
					Kind:       "mykind",
					Namespace:  "mynamespace",
					Name:       "myname",
					APIVersion: "myapiversion",
				},
				URI:     apis.HTTP("dls.example.com"),
				CACerts: &dlsCACerts,
			},
			Retry:         pointer.Int32(3),
			BackoffPolicy: &linear,
			BackoffDelay:  &delay,
		},
	}
	want := Subscription{
		Subscriber: duckv1.Addressable{
			URL:     apis.HTTP("subscriber.example.com"),
			CACerts: &subscriberCACerts,
		},
		Reply: &duckv1.Addressable{
			URL:     apis.HTTP("reply.example.com"),
			CACerts: &replyCACerts,
		},
		DeadLetter: &duckv1.Addressable{
			URL:     apis.HTTP("dls.example.com"),
			CACerts: &dlsCACerts,
		},
		RetryConfig: &kncloudevents.RetryConfig{
			RetryMax:      3,
			BackoffPolicy: &linear,
			BackoffDelay:  &delay,
		},
	}
	got, err := SubscriberSpecToFanoutConfig(*spec)
	if err != nil {
		t.Error("Failed to convert using SubscriberSpecToFanoutConfig:", err)
	}
	if diff := cmp.Diff(&want, got, cmpopts.IgnoreFields(kncloudevents.RetryConfig{}, "Backoff", "CheckRetry")); diff != "" {
		t.Error("Unexpected diff", diff)
	}
}

func TestGetSetSubscriptions(t *testing.T) {
	h := &FanoutEventHandler{subscriptions: make([]Subscription, 0)}
	subs := h.GetSubscriptions(context.TODO())
	if len(subs) != 0 {
		t.Error("Wanted 0 subs, got: ", len(subs))
	}
	h.SetSubscriptions(context.TODO(), []Subscription{{Subscriber: duckv1.Addressable{
		URL: apis.HTTP("subscriber.example.com"),
	}}})
	subs = h.GetSubscriptions(context.TODO())
	if len(subs) != 1 {
		t.Error("Wanted 1 subs, got: ", len(subs))

	}
	h.SetSubscriptions(context.TODO(), []Subscription{{Subscriber: duckv1.Addressable{
		URL: apis.HTTP("subscriber.example.com"),
	}}, {Subscriber: duckv1.Addressable{
		URL: apis.HTTP("subscriber2.example.com"),
	}}})
	subs = h.GetSubscriptions(context.TODO())
	if len(subs) != 2 {
		t.Error("Wanted 2 subs, got: ", len(subs))

	}
	h.SetSubscriptions(context.TODO(), []Subscription{{Subscriber: duckv1.Addressable{
		URL: apis.HTTP("subscriber.example.com"),
	}}, {Subscriber: duckv1.Addressable{
		URL: apis.HTTP("subscriber3.example.com"),
	}}})
	subs = h.GetSubscriptions(context.TODO())
	if len(subs) != 2 {
		t.Error("Wanted 2 subs, got: ", len(subs))

	}

}

func TestFanoutEventHandler_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		receiverFunc        channel.EventReceiverFunc
		timeout             time.Duration
		subs                []Subscription
		subscriber          func(http.ResponseWriter, *http.Request)
		subscriberReqs      int
		replier             func(http.ResponseWriter, *http.Request)
		replierReqs         int
		expectedStatus      int
		asyncExpectedStatus int
	}{
		"rejected by receiver": {
			receiverFunc: func(context.Context, channel.ChannelReference, event.Event, http.Header) error {
				return errors.New("rejected by test-receiver")
			},
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusInternalServerError,
		},
		"receiver has span": {
			receiverFunc: func(ctx context.Context, _ channel.ChannelReference, _ event.Event, _ http.Header) error {
				if span := trace.FromContext(ctx); span == nil {
					return errors.New("missing span")
				}
				return nil
			},
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"fanout times out": {
			timeout: time.Millisecond,
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
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
			subs:                []Subscription{},
			expectedStatus:      http.StatusAccepted,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"reply fails": {
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
				},
			},
			subscriber: func(writer http.ResponseWriter, req *http.Request) {
				// response with some event for reply
				event := test.FullEvent()
				message := binding.ToMessage(&event)
				bindingshttp.WriteResponseWriter(context.TODO(), message, http.StatusAccepted, writer)
				message.Finish(nil)
			},
			subscriberReqs: 1,
			replier: func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNotFound)
			},
			replierReqs:         1,
			expectedStatus:      http.StatusInternalServerError,
			asyncExpectedStatus: http.StatusAccepted,
		},
		"subscriber fails": {
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
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
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
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
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
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
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
				},
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
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
			subs: []Subscription{
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
				},
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
				},
				{
					Subscriber: replaceSubscriber,
					Reply:      &replaceReplier,
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
			testFanoutEventHandler(t, false, tc.receiverFunc, tc.timeout, tc.subs, tc.subscriber, tc.subscriberReqs, tc.replier, tc.replierReqs, tc.expectedStatus)
		})
		t.Run("async - "+n, func(t *testing.T) {
			testFanoutEventHandler(t, true, tc.receiverFunc, tc.timeout, tc.subs, tc.subscriber, tc.subscriberReqs, tc.replier, tc.replierReqs, tc.asyncExpectedStatus)
		})
	}
}

func testFanoutEventHandler(t *testing.T, async bool, receiverFunc channel.EventReceiverFunc, timeout time.Duration, inSubs []Subscription, subscriberHandler func(http.ResponseWriter, *http.Request), subscriberReqs int, replierHandler func(http.ResponseWriter, *http.Request), replierReqs int, expectedStatus int) {
	ctx := context.Background()
	ctx, _ = fakekubeclient.With(ctx)
	ctx = injection.WithConfig(ctx, &rest.Config{})

	var subscriberServerWg *sync.WaitGroup
	reporter := channel.NewStatsReporter("testcontainer", "testpod")
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
	subs := make([]Subscription, 0)
	for _, sub := range inSubs {
		if sub.Subscriber == replaceSubscriber {
			sub.Subscriber = duckv1.Addressable{
				URL: apis.HTTP(subscriberServer.URL[7:]), // strip the leading 'http://'
			}
		}
		if sub.Reply == &replaceReplier {
			sub.Reply = &duckv1.Addressable{
				URL: apis.HTTP(replyServer.URL[7:]), // strip the leading 'http://'
			}
		}
		subs = append(subs, sub)
	}

	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.WarnLevel))
	if err != nil {
		t.Fatal(err)
	}

	oidcTokenProvider := auth.NewOIDCTokenProvider(ctx)
	dispatcher := kncloudevents.NewDispatcher(oidcTokenProvider)

	calledChan := make(chan bool, 1)
	recvOptionFunc := func(*channel.EventReceiver) error {
		calledChan <- true
		return nil
	}

	h, err := NewFanoutEventHandler(
		logger,
		Config{
			Subscriptions: subs,
			AsyncHandler:  async,
		},
		reporter,
		nil,
		nil,
		nil,
		dispatcher,
		recvOptionFunc,
	)
	<-calledChan
	if err != nil {
		t.Fatal("NewHandler failed =", err)
	}

	if receiverFunc != nil {
		receiver, err := channel.NewEventReceiver(receiverFunc, logger, reporter)
		if err != nil {
			t.Fatal("NewEventReceiver failed =", err)
		}
		h.receiver = receiver
	}
	if timeout != 0 {
		h.timeout = timeout
	} else {
		// Reasonable timeout for the tests.
		h.timeout = 10000 * time.Second
	}

	event := makeCloudEvent()
	reqCtx, _ := trace.StartSpan(context.TODO(), "bla")
	req := httptest.NewRequest(http.MethodPost, "http://channelname.channelnamespace/", nil).WithContext(reqCtx)

	ctx = context.Background()

	if err := bindingshttp.WriteRequest(ctx, binding.ToMessage(&event), req); err != nil {
		t.Fatal("WriteRequest =", err)
	}

	resp := httptest.ResponseRecorder{}

	h.ServeHTTP(&resp, req)
	if resp.Code != expectedStatus {
		t.Fatalf("Unexpected status code. Expected %v, Actual %v", expectedStatus, resp.Code)
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

func makeCloudEvent() cloudevents.Event {
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetType("com.example.someevent")
	event.SetSource("/mycontext")
	event.SetID("A234-1234-1234")
	event.SetExtension("comexampleextension", "value")
	event.SetData(cloudevents.ApplicationXML, `<much wow="xml"/>`)
	return event
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
	writer.Header().Set("ce-specversion", cloudevents.VersionV1)
	writer.Header().Set("ce-type", "com.example.someotherevent")
	writer.Header().Set("ce-source", "/myothercontext")
	writer.Header().Set("ce-id", "B234-1234-1234")
	writer.Header().Set("Content-Type", cloudevents.ApplicationJSON)
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte("{}"))
}
