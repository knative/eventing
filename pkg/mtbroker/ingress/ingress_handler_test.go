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
	"bytes"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"

	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	v1 "knative.dev/eventing/pkg/client/listers/eventing/v1"
	broker "knative.dev/eventing/pkg/mtbroker"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

const (
	senderResponseStatusCode = nethttp.StatusAccepted
)

func TestHandler_ServeHTTP(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()

	tt := []struct {
		name            string
		method          string
		uri             string
		body            io.Reader
		headers         nethttp.Header
		expectedHeaders nethttp.Header
		statusCode      int
		handler         nethttp.Handler
		reporter        StatsReporter
		defaulter       client.EventDefaulter
		brokers         []runtime.Object
	}{
		{
			name:       "invalid method PATCH",
			method:     nethttp.MethodPatch,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusMethodNotAllowed,
			handler:    handler(),
			reporter:   &mockReporter{},
			defaulter:  broker.TTLDefaulter(logger, 100),
		},
		{
			name:       "invalid method PUT",
			method:     nethttp.MethodPut,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusMethodNotAllowed,
			handler:    handler(),
			reporter:   &mockReporter{},
			defaulter:  broker.TTLDefaulter(logger, 100),
		},
		{
			name:       "invalid method DELETE",
			method:     nethttp.MethodDelete,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusMethodNotAllowed,
			handler:    handler(),
			reporter:   &mockReporter{},
			defaulter:  broker.TTLDefaulter(logger, 100),
		},
		{
			name:       "invalid method GET",
			method:     nethttp.MethodGet,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusMethodNotAllowed,
			handler:    handler(),
			reporter:   &mockReporter{},
			defaulter:  broker.TTLDefaulter(logger, 100),
		},
		{
			name:       "invalid method OPTIONS",
			method:     nethttp.MethodOptions,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusMethodNotAllowed,
			handler:    handler(),
			reporter:   &mockReporter{},
			defaulter:  broker.TTLDefaulter(logger, 100),
		},
		{
			name:       "valid (happy path)",
			method:     nethttp.MethodPost,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: senderResponseStatusCode,
			handler:    handler(),
			reporter:   &mockReporter{StatusCode: senderResponseStatusCode, EventDispatchTimeReported: true},
			defaulter:  broker.TTLDefaulter(logger, 100),
			brokers: []runtime.Object{
				makeBroker("name", "ns", "http://name-kne-trigger-kn-channel.ns.svc.cluster.local/"),
			},
		},
		{
			name:       "no TTL drop event",
			method:     nethttp.MethodPost,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusBadRequest,
			handler:    handler(),
			reporter:   &mockReporter{StatusCode: nethttp.StatusBadRequest},
			brokers: []runtime.Object{
				makeBroker("name", "ns", "http://name-kne-trigger-kn-channel.ns.svc.cluster.local/"),
			},
		},
		{
			name:       "malformed request URI",
			method:     nethttp.MethodPost,
			uri:        "/knative/ns/name",
			body:       getValidEvent(),
			statusCode: nethttp.StatusBadRequest,
			handler:    handler(),
			reporter:   &mockReporter{},
			brokers: []runtime.Object{
				makeBroker("name", "ns", "http://name-kne-trigger-kn-channel.ns.svc.cluster.local/"),
			},
		},
		{
			name:       "root request URI",
			method:     nethttp.MethodPost,
			uri:        "/",
			body:       getValidEvent(),
			statusCode: nethttp.StatusNotFound,
			handler:    handler(),
			reporter:   &mockReporter{},
			brokers: []runtime.Object{
				makeBroker("name", "ns", "http://name-kne-trigger-kn-channel.ns.svc.cluster.local/"),
			},
		},
		{
			name:       "pass headers to handler",
			method:     nethttp.MethodPost,
			uri:        "/ns/name",
			body:       getValidEvent(),
			statusCode: senderResponseStatusCode,
			headers: nethttp.Header{
				"foo":              []string{"bar"},
				"Traceparent":      []string{"0"},
				"Knative-Foo":      []string{"123"},
				"X-Request-Id":     []string{"123"},
				cehttp.ContentType: []string{event.ApplicationCloudEventsJSON},
			},
			handler: &svc{},
			expectedHeaders: nethttp.Header{
				"Knative-Foo":  []string{"123"},
				"X-Request-Id": []string{"123"},
			},
			reporter:  &mockReporter{StatusCode: senderResponseStatusCode, EventDispatchTimeReported: true},
			defaulter: broker.TTLDefaulter(logger, 100),
			brokers: []runtime.Object{
				makeBroker("name", "ns", "http://name-kne-trigger-kn-channel.ns.svc.cluster.local/"),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {

			s := httptest.NewServer(tc.handler)
			defer s.Close()

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.uri, tc.body)
			if tc.headers != nil {
				request.Header = tc.headers
			} else {
				tc.expectedHeaders = nethttp.Header{
					cehttp.ContentType: []string{event.ApplicationCloudEventsJSON},
				}
				request.Header.Add(cehttp.ContentType, event.ApplicationCloudEventsJSON)
			}

			sender, _ := kncloudevents.NewHttpMessageSender(nil, "")
			h := &Handler{
				Sender:       sender,
				Defaulter:    tc.defaulter,
				Reporter:     &mockReporter{},
				Logger:       logger,
				BrokerLister: &mockBrokerLister{}, //listers.GetV1BrokerLister(),
			}

			h.ServeHTTP(recorder, request)

			result := recorder.Result()
			if result.StatusCode != tc.statusCode {
				t.Errorf("expected status code %d got %d", tc.statusCode, result.StatusCode)
				broker, _ := h.BrokerLister.Brokers("ns").Get("name")
				t.Errorf(broker.Status.Annotations["channelAddress"])
			}

			if svc, ok := tc.handler.(*svc); ok {
				for k, expValue := range tc.expectedHeaders {
					if v, ok := svc.receivedHeaders[k]; !ok {
						t.Errorf("expected header %s - %v", k, svc.receivedHeaders)
					} else if diff := cmp.Diff(expValue, v); diff != "" {
						t.Errorf("(-want +got) %s", diff)
					}
				}
			}

			if diff := cmp.Diff(tc.reporter, h.Reporter); diff != "" {
				t.Errorf("expected reporter state %+v got %+v - diff %s", tc.reporter, h.Reporter, diff)
			}
		})
	}
}

type svc struct {
	receivedHeaders nethttp.Header
}

func (s *svc) ServeHTTP(w nethttp.ResponseWriter, req *nethttp.Request) {
	s.receivedHeaders = req.Header
	w.WriteHeader(senderResponseStatusCode)
}

func handler() nethttp.Handler {
	return nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
		writer.WriteHeader(senderResponseStatusCode)
	})
}

type mockReporter struct {
	StatusCode                int
	EventDispatchTimeReported bool
}

func (r *mockReporter) ReportEventCount(_ *ReportArgs, responseCode int) error {
	r.StatusCode = responseCode
	return nil
}

func (r *mockReporter) ReportEventDispatchTime(_ *ReportArgs, _ int, _ time.Duration) error {
	r.EventDispatchTimeReported = true
	return nil
}

func getValidEvent() io.Reader {
	e := event.New()
	e.SetType("type")
	e.SetSource("source")
	e.SetID("1234")
	b, _ := e.MarshalJSON()
	return bytes.NewBuffer(b)
}

type mockBrokerNamespaceLister struct {
	namespace string
}

func (r *mockBrokerNamespaceLister) List(selector labels.Selector) ([]*eventingv1.Broker, error) {
	return nil, nil
}

func (r *mockBrokerNamespaceLister) Get(name string) (*eventingv1.Broker, error) {
	address := guessChannelAddress(name, r.namespace, utils.GetClusterDomainName())
	return makeBroker(name, r.namespace, address), nil
}

type mockBrokerLister struct {
}

func (r *mockBrokerLister) List(selector labels.Selector) ([]*eventingv1.Broker, error) {
	return nil, nil
}

func (r *mockBrokerLister) Brokers(namespace string) v1.BrokerNamespaceLister {
	return &mockBrokerNamespaceLister{namespace}
}

func makeBroker(name, namespace, channelAddress string) *eventingv1.Broker {
	return &eventingv1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       "broker-uid",
			Annotations: map[string]string{
				eventing.BrokerClassKey: eventing.MTChannelBrokerClassValue,
			},
		},
		Spec: eventingv1.BrokerSpec{
			Config: &duckv1.KReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  "knative-eventing",
				Name:       "imc-channel",
			},
		},
		Status: eventingv1.BrokerStatus{
			Status: duckv1.Status{
				Annotations: map[string]string{
					"channelAddress": channelAddress,
				},
			},
			Address: duckv1.Addressable{
				URL: apis.HTTP(""),
			},
		},
	}
}
