/*
Copyright 2019 The Knative Authors

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

package filter

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/apis"

	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	broker "knative.dev/eventing/pkg/mtbroker"
	reconcilertesting "knative.dev/eventing/pkg/reconciler/testing"
)

const (
	testNS         = "test-namespace"
	triggerName    = "test-trigger"
	triggerUID     = "test-trigger-uid"
	eventType      = `com.example.someevent`
	eventSource    = `/mycontext`
	extensionName  = `myextension`
	extensionValue = `my-extension-value`

	// Because it's a URL we're comparing to, without protocol it looks like this.
	toBeReplaced = "//toBeReplaced"

	invalidEvent = `{"id":"1234","knativebrokerttl":1,"source":"/mycontext","specversion":"0.1","type":"com.example.someevent"}`
)

var (
	validPath = fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID)
)

func init() {
	// Add types to scheme.
	_ = eventingv1beta1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		triggers                    []*eventingv1beta1.Trigger
		request                     *http.Request
		event                       *cloudevents.Event
		requestFails                bool
		failureStatus               int
		returnedEvent               *cloudevents.Event
		expectNewToFail             bool
		expectedDispatch            bool
		expectedStatus              int
		expectedHeaders             http.Header
		expectedEventCount          bool
		expectedEventDispatchTime   bool
		expectedEventProcessingTime bool
		response                    *http.Response
	}{
		"Not POST": {
			request:        httptest.NewRequest(http.MethodGet, validPath, nil),
			expectedStatus: http.StatusMethodNotAllowed,
		},
		"Path too short": {
			request:        httptest.NewRequest(http.MethodPost, "/test-namespace/test-trigger", nil),
			expectedStatus: http.StatusBadRequest,
		},
		"Path too long": {
			request:        httptest.NewRequest(http.MethodPost, "/triggers/test-namespace/test-trigger/extra", nil),
			expectedStatus: http.StatusBadRequest,
		},
		"Path without prefix": {
			request:        httptest.NewRequest(http.MethodPost, "/something/test-namespace/test-trigger", nil),
			expectedStatus: http.StatusBadRequest,
		},
		"Trigger.Get fails": {
			// No trigger exists, so the Get will fail.
			expectedStatus: http.StatusBadRequest,
		},
		"Trigger doesn't have SubscriberURI": {
			triggers: []*eventingv1beta1.Trigger{
				makeTriggerWithoutSubscriberURI(),
			},
			expectedStatus:     http.StatusBadRequest,
			expectedEventCount: true,
		},
		"Trigger without a Filter": {
			triggers: []*eventingv1beta1.Trigger{
				makeTriggerWithoutFilter(),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"No TTL": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("some-other-type", "")),
			},
			event: makeEventWithoutTTL(),
		},
		"Wrong type": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("some-other-type", "")),
			},
			expectedEventCount: false,
		},
		"Wrong type with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("some-other-type", "")),
			},
			expectedEventCount: false,
		},
		"Wrong source": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Wrong source with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Wrong extension": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Dispatch failed": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			requestFails:              true,
			expectedStatus:            http.StatusBadRequest,
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Specific": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes(eventType, eventSource)),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Specific with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes(eventType, eventSource)),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Extension with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, extensionValue)),
			},
			event:                     makeEventWithExtension(extensionName, extensionValue),
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any with attribs - Arrival extension": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			event:                       makeEventWithExtension(broker.EventArrivalTime, "2019-08-26T23:38:17.834384404Z"),
			expectedDispatch:            true,
			expectedEventCount:          true,
			expectedEventDispatchTime:   true,
			expectedEventProcessingTime: true,
		},
		"Wrong Extension with attribs": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, "some-other-extension-value")),
			},
			event:              makeEventWithExtension(extensionName, extensionValue),
			expectedEventCount: false,
		},
		"Returned Cloud Event": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			returnedEvent:             makeDifferentEvent(),
		},
		"Error From Trigger": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			event:                     makeEvent(),
			requestFails:              true,
			failureStatus:             http.StatusTooManyRequests,
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusTooManyRequests,
		},
		"Returned Cloud Event with custom headers": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			request: func() *http.Request {
				e := makeEvent()
				b, _ := e.MarshalJSON()
				request := httptest.NewRequest(http.MethodPost, validPath, bytes.NewBuffer(b))

				// foo won't pass filtering.
				request.Header.Set("foo", "bar")
				// Traceparent will not pass filtering.
				request.Header.Set("Traceparent", "0")
				// Knative-Foo will pass as a prefix match.
				request.Header.Set("Knative-Foo", "baz")
				// X-Request-Id will pass as an exact header match.
				request.Header.Set("X-Request-Id", "123")
				// Content-Type will not pass filtering.
				request.Header.Set(cehttp.ContentType, event.ApplicationCloudEventsJSON)

				return request
			}(),
			expectedHeaders: http.Header{
				// X-Request-Id will pass as an exact header match.
				"X-Request-Id": []string{"123"},
				// Knative-Foo will pass as a prefix match.
				"Knative-Foo": []string{"baz"},
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			returnedEvent:             makeDifferentEvent(),
		},
		"Returned non empty non event response": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusBadGateway,
			response:                  makeNonEmptyResponse(),
		},
		"Returned malformed Cloud Event": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusOK,
			response:                  makeMalformedEventResponse(),
		},
		"Returned malformed structured Cloud Event": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusBadGateway,
			response:                  makeMalformedStructuredEventResponse(),
		},
		"Returned empty body 200": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusOK,
			response:                  makeEmptyResponse(200),
		},
		"Returned empty body 202": {
			triggers: []*eventingv1beta1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedStatus:            http.StatusAccepted,
			response:                  makeEmptyResponse(202),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			fh := fakeHandler{
				failRequest:   tc.requestFails,
				failStatus:    tc.failureStatus,
				returnedEvent: tc.returnedEvent,
				headers:       tc.expectedHeaders,
				t:             t,
				response:      tc.response,
			}
			s := httptest.NewServer(&fh)
			defer s.Close()

			// Replace the SubscriberURI to point at our fake server.
			correctURI := make([]runtime.Object, 0, len(tc.triggers))
			for _, trig := range tc.triggers {
				if trig.Status.SubscriberURI != nil && trig.Status.SubscriberURI.String() == toBeReplaced {

					url, err := apis.ParseURL(s.URL)
					if err != nil {
						t.Fatalf("Failed to parse URL %q : %s", s.URL, err)
					}
					trig.Status.SubscriberURI = url
				}
				correctURI = append(correctURI, trig)
			}
			listers := reconcilertesting.NewListers(correctURI)
			reporter := &mockReporter{}
			r, err := NewHandler(
				zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
				listers.GetV1Beta1TriggerLister(),
				reporter,
				8080)
			if tc.expectNewToFail {
				if err == nil {
					t.Fatal("Expected New to fail, it didn't")
				}
				return
			} else if err != nil {
				t.Fatal("Unable to create receiver:", err)
			}

			e := tc.event
			if e == nil {
				e = makeEvent()
			}
			if tc.request == nil {
				b, err := e.MarshalJSON()
				if err != nil {
					t.Fatal(err)
				}
				tc.request = httptest.NewRequest(http.MethodPost, validPath, bytes.NewBuffer(b))
				tc.request.Header.Set(cehttp.ContentType, event.ApplicationCloudEventsJSON)
			}
			responseWriter := httptest.NewRecorder()
			r.ServeHTTP(&responseWriterWithInvocationsCheck{
				ResponseWriter: responseWriter,
				headersWritten: atomic.NewBool(false),
				t:              t,
			}, tc.request)

			response := responseWriter.Result()

			if tc.expectedStatus != 0 && tc.expectedStatus != response.StatusCode {
				t.Errorf("Unexpected status. Expected %v. Actual %v.", tc.expectedStatus, response.StatusCode)
			}
			if tc.expectedDispatch != fh.requestReceived {
				t.Errorf("Incorrect dispatch. Expected %v, Actual %v", tc.expectedDispatch, fh.requestReceived)
			}
			if tc.expectedEventCount != reporter.eventCountReported {
				t.Errorf("Incorrect event count reported metric. Expected %v, Actual %v", tc.expectedEventCount, reporter.eventCountReported)
			}
			if tc.expectedEventDispatchTime != reporter.eventDispatchTimeReported {
				t.Errorf("Incorrect event dispatch time reported metric. Expected %v, Actual %v", tc.expectedEventDispatchTime, reporter.eventDispatchTimeReported)
			}
			if tc.expectedEventProcessingTime != reporter.eventProcessingTimeReported {
				t.Errorf("Incorrect event processing time reported metric. Expected %v, Actual %v", tc.expectedEventProcessingTime, reporter.eventProcessingTimeReported)
			}
			if tc.returnedEvent != nil {
				if tc.returnedEvent.SpecVersion() != event.CloudEventsVersionV1 {
					t.Errorf("Incorrect spec version. Expected %v, Actual %v", tc.returnedEvent.SpecVersion(), event.CloudEventsVersionV1)
				}
			}
			// Compare the returned event.
			message := cehttp.NewMessageFromHttpResponse(response)
			event, err := binding.ToEvent(context.Background(), message)
			if tc.returnedEvent == nil {
				if err == nil || event != nil {
					t.Fatal("Unexpected response event:", event)
				}
				return
			}
			if err != nil || event == nil {
				t.Fatalf("Expected response event, actually nil")
			}

			// The TTL will be added again.
			expectedResponseEvent := addTTLToEvent(*tc.returnedEvent)

			// cloudevents/sdk-go doesn't preserve the extension type, so get TTL and set it back again.
			// https://github.com/cloudevents/sdk-go/blob/97abfeb3da0bed09e395bff2c5bcf35b6435cb5f/v2/types/value.go#L57
			ttl, err := broker.GetTTL(event.Context)
			if err != nil {
				t.Error("failed to get TTL", err)
			}
			err = broker.SetTTL(event.Context, ttl)
			if err != nil {
				t.Error("failed to set TTL", err)
			}

			if diff := cmp.Diff(expectedResponseEvent.Context.AsV1(), event.Context.AsV1()); diff != "" {
				t.Error("Incorrect response event context (-want +got):", diff)
			}
			if diff := cmp.Diff(expectedResponseEvent.Data(), event.Data()); diff != "" {
				t.Error("Incorrect response event data (-want +got):", diff)
			}
		})
	}
}

type responseWriterWithInvocationsCheck struct {
	http.ResponseWriter
	headersWritten *atomic.Bool
	t              *testing.T
}

func (r *responseWriterWithInvocationsCheck) WriteHeader(statusCode int) {
	if !r.headersWritten.CAS(false, true) {
		r.t.Fatal("WriteHeader invoked more than once")
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

type mockReporter struct {
	eventCountReported          bool
	eventDispatchTimeReported   bool
	eventProcessingTimeReported bool
}

func (r *mockReporter) ReportEventCount(args *ReportArgs, responseCode int) error {
	r.eventCountReported = true
	return nil
}

func (r *mockReporter) ReportEventDispatchTime(args *ReportArgs, responseCode int, d time.Duration) error {
	r.eventDispatchTimeReported = true
	return nil
}

func (r *mockReporter) ReportEventProcessingTime(args *ReportArgs, d time.Duration) error {
	r.eventProcessingTimeReported = true
	return nil
}

type fakeHandler struct {
	failRequest     bool
	failStatus      int
	requestReceived bool
	headers         http.Header
	returnedEvent   *cloudevents.Event
	t               *testing.T
	response        *http.Response
}

func (h *fakeHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if h.returnedEvent != nil && h.response != nil {
		h.t.Errorf("Can not specify both returnedEvent and response.")
	}
	h.requestReceived = true

	for n, v := range h.headers {
		if strings.Contains(strings.ToLower(n), strings.ToLower(broker.TTLAttribute)) {
			h.t.Errorf("Broker TTL should not be seen by the subscriber: %s", n)
		}
		if diff := cmp.Diff(v, req.Header[n]); diff != "" {
			h.t.Errorf("Incorrect request header '%s' (-want +got): %s", n, diff)
		}
	}

	if h.failRequest {
		if h.failStatus != 0 {
			resp.WriteHeader(h.failStatus)
		} else {
			resp.WriteHeader(http.StatusBadRequest)
		}
		return
	}
	if h.returnedEvent == nil && h.response == nil {
		resp.WriteHeader(http.StatusAccepted)
		return
	}

	if h.returnedEvent != nil {
		message := binding.ToMessage(h.returnedEvent)
		defer message.Finish(nil)
		err := cehttp.WriteResponseWriter(context.Background(), message, http.StatusAccepted, resp)
		if err != nil {
			h.t.Fatalf("Unable to write body: %v", err)
		}
	}
	if h.response != nil {
		for k, v := range h.response.Header {
			resp.Header().Set(k, v[0])
		}
		resp.WriteHeader(h.response.StatusCode)
		if h.response.Body != nil {
			defer h.response.Body.Close()
			body, err := ioutil.ReadAll(h.response.Body)
			if err != nil {
				h.t.Fatal("Unable to read body: ", err)
			}
			resp.Write(body)
		}
	}
}

func makeTriggerFilterWithAttributes(t, s string) *eventingv1beta1.TriggerFilter {
	return &eventingv1beta1.TriggerFilter{
		Attributes: eventingv1beta1.TriggerFilterAttributes{
			"type":   t,
			"source": s,
		},
	}
}

func makeTriggerFilterWithAttributesAndExtension(t, s, e string) *eventingv1beta1.TriggerFilter {
	return &eventingv1beta1.TriggerFilter{
		Attributes: eventingv1beta1.TriggerFilterAttributes{
			"type":        t,
			"source":      s,
			extensionName: e,
		},
	}
}

func makeTrigger(filter *eventingv1beta1.TriggerFilter) *eventingv1beta1.Trigger {
	return &eventingv1beta1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1beta1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: eventingv1beta1.TriggerSpec{
			Filter: filter,
		},
		Status: eventingv1beta1.TriggerStatus{
			SubscriberURI: &apis.URL{Host: "toBeReplaced"},
		},
	}
}

func makeTriggerWithoutFilter() *eventingv1beta1.Trigger {
	t := makeTrigger(makeTriggerFilterWithAttributes("", ""))
	t.Spec.Filter = nil
	return t
}

func makeTriggerWithoutSubscriberURI() *eventingv1beta1.Trigger {
	t := makeTrigger(makeTriggerFilterWithAttributes("", ""))
	t.Status = eventingv1beta1.TriggerStatus{}
	return t
}

func makeEventWithoutTTL() *cloudevents.Event {
	e := event.New()
	e.SetType(eventType)
	e.SetSource(eventSource)
	e.SetID("1234")
	return &e
}

func makeEvent() *cloudevents.Event {
	noTTL := makeEventWithoutTTL()
	e := addTTLToEvent(*noTTL)
	return &e
}

func addTTLToEvent(e cloudevents.Event) cloudevents.Event {
	_ = broker.SetTTL(e.Context, 1)
	return e
}

func makeDifferentEvent() *cloudevents.Event {
	e := makeEvent()
	e.SetSource("another-source")
	e.SetID("another-id")
	return e
}

func makeEventWithExtension(extName, extValue string) *cloudevents.Event {
	noTTL := makeEvent()
	noTTL.SetExtension(extName, extValue)
	e := addTTLToEvent(*noTTL)
	return &e
}

func makeNonEmptyResponse() *http.Response {
	r := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Body:       ioutil.NopCloser(bytes.NewBufferString(invalidEvent)),
		Header:     make(http.Header),
	}
	r.Header.Set("Content-Type", "garbage")
	r.Header.Set("Content-Length", fmt.Sprintf("%d", len(invalidEvent)))
	return r
}

func makeMalformedEventResponse() *http.Response {
	r := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	r.Header.Set("Ce-Specversion", "9000.1")
	return r
}

func makeMalformedStructuredEventResponse() *http.Response {
	r := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
		Header:     make(http.Header),
	}
	r.Header.Set("Content-Type", cloudevents.ApplicationCloudEventsJSON)

	return r
}

func makeEmptyResponse(status int) *http.Response {
	s := fmt.Sprintf("%d OK", status)
	r := &http.Response{
		Status:     s,
		StatusCode: status,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	return r
}
