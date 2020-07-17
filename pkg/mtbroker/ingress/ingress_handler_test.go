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

	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	broker "knative.dev/eventing/pkg/mtbroker"
	reconcilertestingv1 "knative.dev/eventing/pkg/reconciler/testing/v1"
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
		brokers         []*eventingv1.Broker
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
			brokers: []*eventingv1.Broker{
				makeBroker("name", "ns"),
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
			brokers: []*eventingv1.Broker{
				makeBroker("name", "ns"),
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
			brokers: []*eventingv1.Broker{
				makeBroker("name", "ns"),
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
			brokers: []*eventingv1.Broker{
				makeBroker("name", "ns"),
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
			brokers: []*eventingv1.Broker{
				makeBroker("name", "ns"),
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

			annotatedBrokers := make([]runtime.Object, 0, len(tc.brokers))
			for _, b := range tc.brokers {
				// Write the channel address in the broker status annotation
				if b.Status.Annotations == nil {
					b.Status.Annotations = make(map[string]string, 1)
				}
				b.Status.Annotations["channelAddress"] = s.URL
				annotatedBrokers = append(annotatedBrokers, b)
			}
			listers := reconcilertestingv1.NewListers(annotatedBrokers)
			sender, _ := kncloudevents.NewHttpMessageSender(nil, "")
			h := &Handler{
				Sender:       sender,
				Defaulter:    tc.defaulter,
				Reporter:     &mockReporter{},
				Logger:       logger,
				BrokerLister: listers.GetBrokerLister(),
			}

			h.ServeHTTP(recorder, request)

			result := recorder.Result()
			if result.StatusCode != tc.statusCode {
				t.Errorf("expected status code %d got %d", tc.statusCode, result.StatusCode)
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

func makeBroker(name, namespace string) *eventingv1.Broker {
	return &eventingv1.Broker{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1",
			Kind:       "Broker",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: eventingv1.BrokerSpec{},
	}
}
