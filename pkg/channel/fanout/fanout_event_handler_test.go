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

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/observability/tracing"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/kncloudevents"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	bindingshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/apis"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/pkg/channel"
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
				if span := trace.SpanFromContext(ctx); span == nil {
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
	dispatcher := kncloudevents.NewDispatcher(eventingtls.NewDefaultClientConfig(), oidcTokenProvider)

	calledChan := make(chan bool, 1)
	recvOptionFunc := func(*channel.EventReceiver) error {
		calledChan <- true
		return nil
	}

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTextMapPropagator(tracing.DefaultTextMapPropagator())

	h, err := NewFanoutEventHandler(
		logger,
		Config{
			Subscriptions: subs,
			AsyncHandler:  async,
		},
		nil,
		nil,
		nil,
		dispatcher,
		mp,
		tp,
		recvOptionFunc,
	)
	<-calledChan
	if err != nil {
		t.Fatal("NewHandler failed =", err)
	}

	if receiverFunc != nil {
		receiver, err := channel.NewEventReceiver(
			receiverFunc,
			logger,
			channel.MeterProvider(mp),
			channel.TraceProvider(tp),
		)
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
	tracer := tp.Tracer(ScopeName)
	reqCtx, _ := tracer.Start(context.TODO(), "bla")
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

func TestFanoutEventHandlerDispatchDurationWithDeadLetterSink(t *testing.T) {
	testCases := map[string]struct {
		deadLetterStatus int
		expectedStatus   int
	}{
		"dead letter sink accepts the event": {
			deadLetterStatus: http.StatusAccepted,
			expectedStatus:   http.StatusAccepted,
		},
		"dead letter sink fails": {
			deadLetterStatus: http.StatusInternalServerError,
			expectedStatus:   http.StatusInternalServerError,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			const subscriberDelay = 100 * time.Millisecond
			subscriber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(subscriberDelay)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer subscriber.Close()
			healthySubscriber := httptest.NewServer(http.HandlerFunc(callableSucceed))
			defer healthySubscriber.Close()
			deadLetterSink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.deadLetterStatus)
			}))
			defer deadLetterSink.Close()

			reader := metric.NewManualReader()
			mp := metric.NewMeterProvider(metric.WithReader(reader))
			defer mp.Shutdown(context.Background())
			dispatcher := kncloudevents.NewDispatcher(eventingtls.NewDefaultClientConfig(), nil)
			subscriptions := []Subscription{
				{
					Name:       "failing",
					Namespace:  "channelnamespace",
					Subscriber: duckv1.Addressable{URL: apis.HTTP(subscriber.Listener.Addr().String())},
					DeadLetter: &duckv1.Addressable{URL: apis.HTTP(deadLetterSink.Listener.Addr().String())},
				},
				{
					Name:       "healthy",
					Namespace:  "channelnamespace",
					Subscriber: duckv1.Addressable{URL: apis.HTTP(healthySubscriber.Listener.Addr().String())},
				},
			}
			h, err := NewFanoutEventHandler(zap.NewNop(), Config{Subscriptions: subscriptions}, nil, nil, nil, dispatcher, mp, nil)
			if err != nil {
				t.Fatal(err)
			}

			labeler := &otelhttp.Labeler{}
			ctx := otelhttp.ContextWithLabeler(t.Context(), labeler)
			req := httptest.NewRequestWithContext(ctx, http.MethodPost, "http://channelname.channelnamespace/", nil)
			event := makeCloudEvent()
			if err := bindingshttp.WriteRequest(ctx, binding.ToMessage(&event), req); err != nil {
				t.Fatal(err)
			}
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != tc.expectedStatus {
				t.Fatalf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Code)
			}

			var points []metricdata.HistogramDataPoint[float64]
			for start := time.Now(); len(points) < len(subscriptions) && time.Since(start) < 5*time.Second; time.Sleep(10 * time.Millisecond) {
				points = dispatchDurationPoints(t, reader)
			}
			if len(points) != len(subscriptions) {
				t.Fatalf("Expected %d dispatch duration series, got %d", len(subscriptions), len(points))
			}

			wantStatuses := map[string]int64{
				"subscription:failing.channelnamespace": http.StatusServiceUnavailable,
				"subscription:healthy.channelnamespace": http.StatusOK,
			}
			gotStatuses := make(map[string]int64, len(points))
			for _, point := range points {
				destination, _ := point.Attributes.Value(semconv.MessagingDestinationNameKey)
				status, _ := point.Attributes.Value(semconv.HTTPResponseStatusCodeKey)
				gotStatuses[destination.AsString()] = status.AsInt64()
				if point.Count != 1 {
					t.Errorf("Expected 1 dispatch for %s, got %d", destination.AsString(), point.Count)
				}
				if destination.AsString() == "subscription:failing.channelnamespace" && point.Sum < subscriberDelay.Seconds() {
					t.Errorf("Expected failed dispatch lasting at least %v, got %vs", subscriberDelay, point.Sum)
				}
			}
			if diff := cmp.Diff(wantStatuses, gotStatuses); diff != "" {
				t.Errorf("Unexpected dispatch statuses (-want +got):\n%s", diff)
			}
			for _, attr := range labeler.Get() {
				if attr.Key == semconv.HTTPResponseStatusCodeKey {
					t.Errorf("Found a subscription status in the shared request labeler: %v", attr.Value)
				}
			}
		})
	}
}

func TestFanoutDoesNotRecordDispatchDurationWhenTokenRequestFails(t *testing.T) {
	ctx, client := fakekubeclient.With(t.Context())
	client.PrependReactor("create", "serviceaccounts", func(action kubetesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "token" {
			return true, nil, errors.New("token request failed")
		}
		return false, nil, nil
	})

	subscriberRequests := atomic.NewInt32(0)
	subscriber := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		subscriberRequests.Inc()
		w.WriteHeader(http.StatusOK)
	}))
	defer subscriber.Close()
	deadLetterRequests := atomic.NewInt32(0)
	deadLetterSink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		deadLetterRequests.Inc()
		w.WriteHeader(http.StatusAccepted)
	}))
	defer deadLetterSink.Close()

	sub := Subscription{
		Name:      "oidc",
		Namespace: "channelnamespace",
		Subscriber: duckv1.Addressable{
			URL:      apis.HTTP(subscriber.Listener.Addr().String()),
			Audience: pointer.String("subscriber-audience"),
		},
		DeadLetter:     &duckv1.Addressable{URL: apis.HTTP(deadLetterSink.Listener.Addr().String())},
		ServiceAccount: &types.NamespacedName{Name: "delivery", Namespace: "channelnamespace"},
	}
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer mp.Shutdown(context.Background())
	dispatcher := kncloudevents.NewDispatcher(eventingtls.NewDefaultClientConfig(), auth.NewOIDCTokenProvider(ctx))
	h, err := NewFanoutEventHandler(zap.NewNop(), Config{}, nil, nil, nil, dispatcher, mp, nil)
	if err != nil {
		t.Fatal(err)
	}

	info, err := h.makeFanoutRequest(ctx, makeCloudEvent(), nil, sub)
	if err != nil {
		t.Fatalf("Sending to the dead letter sink failed: %v", err)
	}
	if got := subscriberRequests.Load(); got != 0 {
		t.Errorf("Expected no requests to the subscriber, got %d", got)
	}
	if got := deadLetterRequests.Load(); got != 1 {
		t.Errorf("Expected 1 request to the dead letter sink, got %d", got)
	}

	h.recordDispatchDuration(ctx, sub, info)
	if points := dispatchDurationPoints(t, reader); len(points) != 0 {
		t.Errorf("Expected no dispatch duration samples, got %v", points)
	}
}

func dispatchDurationPoints(t *testing.T, reader *metric.ManualReader) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name != ScopeName {
			continue
		}
		for _, m := range sm.Metrics {
			if m.Name == "kn.eventing.dispatch.duration" {
				return m.Data.(metricdata.Histogram[float64]).DataPoints
			}
		}
	}
	return nil
}
