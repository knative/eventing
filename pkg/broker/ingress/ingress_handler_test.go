/*
 * Copyright 2019 The Knative Authors
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

package ingress

import (
	"context"
	nethttp "net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"knative.dev/eventing/pkg/broker"
)

const (
	namespace       = "testNamespace"
	brokerName      = "testBroker"
	validURI        = "/"
	urlHost         = "testHost"
	urlPath         = "/"
	urlScheme       = "http"
	validHTTPMethod = nethttp.MethodPost
)

type mockReporter struct {
	eventCountReported        bool
	eventDispatchTimeReported bool
}

func (r *mockReporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	r.eventCountReported = true
	return nil
}

func (r *mockReporter) ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error {
	r.eventDispatchTimeReported = true
	return nil
}

type fakeClient struct {
	sent bool
	fn   interface{}
	mux  sync.Mutex
}

func (f *fakeClient) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	f.sent = true
	return ctx, &event, nil
}

func (f *fakeClient) StartReceiver(ctx context.Context, fn interface{}) error {
	f.mux.Lock()
	f.fn = fn
	f.mux.Unlock()
	<-ctx.Done()
	return nil
}

func (f *fakeClient) ready() bool {
	f.mux.Lock()
	ready := f.fn != nil
	f.mux.Unlock()
	return ready
}

func (f *fakeClient) fakeReceive(t *testing.T, event cloudevents.Event) {
	// receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error

	resp := new(cloudevents.EventResponse)
	tctx := http.TransportContext{Header: nethttp.Header{}, Method: validHTTPMethod, URI: validURI}
	ctx := http.WithTransportContext(context.Background(), tctx)

	fnType := reflect.TypeOf(f.fn)
	if fnType.Kind() != reflect.Func {
		t.Fatal("wrong method type.", fnType.Kind())
	}

	fn := reflect.ValueOf(f.fn)
	_ = fn.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(event), reflect.ValueOf(resp)})
}

func TestIngressHandler_Receive_FAIL(t *testing.T) {
	testCases := map[string]struct {
		httpmethod                string
		URI                       string
		expectedStatus            int
		expectedEventCount        bool
		expectedEventDispatchTime bool
	}{
		"method not allowed": {
			httpmethod:     nethttp.MethodGet,
			URI:            validURI,
			expectedStatus: nethttp.StatusMethodNotAllowed,
		},
		"invalid url": {
			httpmethod:     validHTTPMethod,
			URI:            "invalidURI",
			expectedStatus: nethttp.StatusNotFound,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			client, _ := cloudevents.NewDefaultClient()
			reporter := &mockReporter{}
			handler := Handler{
				Logger:   zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
				CeClient: client,
				ChannelURI: &url.URL{
					Scheme: urlScheme,
					Host:   urlHost,
					Path:   urlPath,
				},
				BrokerName: brokerName,
				Namespace:  namespace,
				Reporter:   reporter,
				Defaulter:  broker.TTLDefaulter(zap.NewNop(), 5),
			}
			event := cloudevents.NewEvent(cloudevents.VersionV1)
			resp := new(cloudevents.EventResponse)
			tctx := http.TransportContext{Header: nethttp.Header{}, Method: tc.httpmethod, URI: tc.URI}
			ctx := http.WithTransportContext(context.Background(), tctx)
			_ = handler.receive(ctx, event, resp)
			if resp.Status != tc.expectedStatus {
				t.Errorf("Unexpected status code. Expected %v, Actual %v", tc.expectedStatus, resp.Status)
			}
			if reporter.eventCountReported != tc.expectedEventCount {
				t.Errorf("Unexpected event count reported. Expected %v, Actual %v", tc.expectedEventCount, reporter.eventCountReported)
			}
			if reporter.eventDispatchTimeReported != tc.expectedEventDispatchTime {
				t.Errorf("Unexpected event dispatch time reported. Expected %v, Actual %v", tc.expectedEventDispatchTime, reporter.eventDispatchTimeReported)
			}
		})
	}
}

func TestIngressHandler_Receive_Succeed(t *testing.T) {
	client := &fakeClient{}
	reporter := &mockReporter{}
	handler := Handler{
		Logger:   zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
		CeClient: client,
		ChannelURI: &url.URL{
			Scheme: urlScheme,
			Host:   urlHost,
			Path:   urlPath,
		},
		BrokerName: brokerName,
		Namespace:  namespace,
		Reporter:   reporter,
		Defaulter:  broker.TTLDefaulter(zap.NewNop(), 5),
	}

	event := cloudevents.NewEvent()
	resp := new(cloudevents.EventResponse)
	tctx := http.TransportContext{Header: nethttp.Header{}, Method: validHTTPMethod, URI: validURI}
	ctx := http.WithTransportContext(context.Background(), tctx)
	_ = handler.receive(ctx, event, resp)

	if !client.sent {
		t.Errorf("client should invoke send function")
	}
	if !reporter.eventCountReported {
		t.Errorf("event count should have been reported")
	}
	if !reporter.eventDispatchTimeReported {
		t.Errorf("event dispatch time should have been reported")
	}
}

func TestIngressHandler_Receive_NoTTL(t *testing.T) {
	client := &fakeClient{}
	reporter := &mockReporter{}
	handler := Handler{
		Logger:   zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
		CeClient: client,
		ChannelURI: &url.URL{
			Scheme: urlScheme,
			Host:   urlHost,
			Path:   urlPath,
		},
		BrokerName: brokerName,
		Namespace:  namespace,
		Reporter:   reporter,
	}
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	resp := new(cloudevents.EventResponse)
	tctx := http.TransportContext{Header: nethttp.Header{}, Method: validHTTPMethod, URI: validURI}
	ctx := http.WithTransportContext(context.Background(), tctx)
	_ = handler.receive(ctx, event, resp)
	if client.sent {
		t.Errorf("client should NOT invoke send function")
	}
	if !reporter.eventCountReported {
		t.Errorf("event count should have been reported")
	}
}

func TestIngressHandler_Start(t *testing.T) {
	client := &fakeClient{}
	reporter := &mockReporter{}
	handler := Handler{
		Logger:   zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
		CeClient: client,
		ChannelURI: &url.URL{
			Scheme: urlScheme,
			Host:   urlHost,
			Path:   urlPath,
		},
		BrokerName: brokerName,
		Namespace:  namespace,
		Reporter:   reporter,
		Defaulter:  broker.TTLDefaulter(zap.NewNop(), 5),
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := handler.Start(ctx); err != nil {
			t.Error(err)
		}
	}()
	// Need time for the handler to start up. Wait.
	for !client.ready() {
		time.Sleep(1 * time.Millisecond)
	}

	event := cloudevents.NewEvent()
	client.fakeReceive(t, event)
	cancel()

	if !client.sent {
		t.Errorf("client should invoke send function")
	}
	if !reporter.eventCountReported {
		t.Errorf("event count should have been reported")
	}
	if !reporter.eventDispatchTimeReported {
		t.Errorf("event dispatch time should have been reported")
	}
}
