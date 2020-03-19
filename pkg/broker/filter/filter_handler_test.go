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

package filter

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	cepkg "github.com/cloudevents/sdk-go/v1/cloudevents"
	cehttp "github.com/cloudevents/sdk-go/v1/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	"knative.dev/pkg/apis"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/client/clientset/versioned/fake"
	"knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/utils"
)

const (
	testNS         = "test-namespace"
	triggerName    = "test-trigger"
	triggerUID     = "test-trigger-uid"
	eventType      = `com.example.someevent`
	eventSource    = `/mycontext`
	extensionName  = `my-extension`
	extensionValue = `my-extension-value`

	// Because it's a URL we're comparing to, without protocol it looks like this.
	toBeReplaced = "//toBeReplaced"
)

var (
	host      = fmt.Sprintf("%s.%s.triggers.%s", triggerName, testNS, utils.GetClusterDomainName())
	validPath = fmt.Sprintf("/triggers/%s/%s/%s", testNS, triggerName, triggerUID)
)

func init() {
	// Add types to scheme.
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		triggers                    []*eventingv1alpha1.Trigger
		tctx                        *cloudevents.HTTPTransportContext
		event                       *cloudevents.Event
		requestFails                bool
		failureStatus               int
		returnedEvent               *cloudevents.Event
		expectNewToFail             bool
		expectedErr                 bool
		expectedDispatch            bool
		expectedStatus              int
		expectedHeaders             http.Header
		expectedEventCount          bool
		expectedEventDispatchTime   bool
		expectedEventProcessingTime bool
	}{
		"Not POST": {
			tctx: &cloudevents.HTTPTransportContext{
				Method: "GET",
				Host:   host,
				URI:    validPath,
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		"Path too short": {
			tctx: &cloudevents.HTTPTransportContext{
				Method: "POST",
				Host:   host,
				URI:    "/test-namespace/test-trigger",
			},
			expectedErr: true,
		},
		"Path too long": {
			tctx: &cloudevents.HTTPTransportContext{
				Method: "POST",
				Host:   host,
				URI:    "/triggers/test-namespace/test-trigger/extra",
			},
			expectedErr: true,
		},
		"Path without prefix": {
			tctx: &cloudevents.HTTPTransportContext{
				Method: "POST",
				Host:   host,
				URI:    "/something/test-namespace/test-trigger",
			},
			expectedErr: true,
		},
		"Bad host": {
			tctx: &cloudevents.HTTPTransportContext{
				Method: "POST",
				Host:   "badhost-cant-be-parsed-as-a-trigger-name-plus-namespace",
				URI:    validPath,
			},
			expectedErr: true,
		},
		"Trigger.Get fails": {
			// No trigger exists, so the Get will fail.
			expectedErr: true,
		},
		"Trigger doesn't have SubscriberURI": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTriggerWithoutSubscriberURI(),
			},
			expectedErr:        true,
			expectedEventCount: true,
		},
		"Trigger without a Filter": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTriggerWithoutFilter(),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"No TTL": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			event: makeEventWithoutTTL(),
		},
		"Wrong type": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("some-other-type", "")),
			},
			expectedEventCount: false,
		},
		"Wrong type with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("some-other-type", "")),
			},
			expectedEventCount: false,
		},
		"Wrong source": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Wrong source with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Wrong extension": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
			expectedEventCount: false,
		},
		"Dispatch failed": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			requestFails:              true,
			expectedErr:               true,
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Specific": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType(eventType, eventSource)),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Specific with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes(eventType, eventSource)),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Extension with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, extensionValue)),
			},
			event:                     makeEventWithExtension(extensionName, extensionValue),
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
		},
		"Dispatch succeeded - Any with attribs - Arrival extension": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			event:                       makeEventWithExtension(broker.EventArrivalTime, "2019-08-26T23:38:17.834384404Z"),
			expectedDispatch:            true,
			expectedEventCount:          true,
			expectedEventDispatchTime:   true,
			expectedEventProcessingTime: true,
		},
		"Wrong Extension with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, "some-other-extension-value")),
			},
			event:              makeEventWithExtension(extensionName, extensionValue),
			expectedEventCount: false,
		},
		"Returned Cloud Event": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			returnedEvent:             makeDifferentEvent(),
		},
		"Error From Trigger": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			tctx: &cloudevents.HTTPTransportContext{
				Method:     "POST",
				Host:       host,
				URI:        validPath,
				StatusCode: http.StatusTooManyRequests,
			},
			requestFails:              true,
			failureStatus:             http.StatusTooManyRequests,
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			expectedErr:               true,
			expectedStatus:            http.StatusTooManyRequests,
		},
		"Returned Cloud Event with custom headers": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			tctx: &cloudevents.HTTPTransportContext{
				Method: "POST",
				Host:   host,
				URI:    validPath,
				Header: http.Header{
					// foo won't pass filtering.
					"foo": []string{"bar"},
					// b3 will not pass filtering.
					"B3": []string{"0"},
					// X-B3-Foo will not pass filtering.
					"X-B3-Foo": []string{"abc"},
					// X-Ot-Foo will not pass filtering.
					"X-Ot-Foo": []string{"haden"},
					// Knative-Foo will pass as a prefix match.
					"Knative-Foo": []string{"baz", "qux"},
					// X-Request-Id will pass as an exact header match.
					"X-Request-Id": []string{"123"},
				},
			},
			expectedHeaders: http.Header{
				// X-Request-Id will pass as an exact header match.
				"X-Request-Id": []string{"123"},
				// Knative-Foo will pass as a prefix match.
				"Knative-Foo": []string{"baz", "qux"},
			},
			expectedDispatch:          true,
			expectedEventCount:        true,
			expectedEventDispatchTime: true,
			returnedEvent:             makeDifferentEvent(),
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
			}
			s := httptest.NewServer(&fh)
			defer s.Client()

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
			reporter := &mockReporter{}
			r, err := NewHandler(
				zap.NewNop(),
				getFakeTriggerLister(correctURI),
				reporter)
			if tc.expectNewToFail {
				if err == nil {
					t.Fatal("Expected New to fail, it didn't")
				}
				return
			} else if err != nil {
				t.Fatalf("Unable to create receiver: %v", err)
			}

			tctx := tc.tctx
			if tctx == nil {
				tctx = &cloudevents.HTTPTransportContext{
					Method: http.MethodPost,
					Host:   host,
					URI:    validPath,
				}
			}
			ctx := cehttp.WithTransportContext(context.Background(), *tctx)
			resp := &cloudevents.EventResponse{}
			event := tc.event
			if event == nil {
				event = makeEvent()
			}
			err = r.serveHTTP(ctx, *event, resp)

			if tc.expectedErr && err == nil {
				t.Errorf("Expected an error, received nil")
			} else if !tc.expectedErr && err != nil {
				t.Errorf("Expected no error, received %v", err)
			}

			if tc.expectedStatus != 0 && tc.expectedStatus != resp.Status {
				t.Errorf("Unexpected status. Expected %v. Actual %v.", tc.expectedStatus, resp.Status)
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
				if tc.returnedEvent.SpecVersion() != cepkg.CloudEventsVersionV1 {
					t.Errorf("Incorrect event processing time reported metric. Expected %v, Actual %v", tc.expectedEventProcessingTime, reporter.eventProcessingTimeReported)
				}
			}
			// Compare the returned event.
			if tc.returnedEvent == nil {
				if resp.Event != nil {
					t.Fatalf("Unexpected response event: %v", resp.Event)
				}
				return
			} else if resp.Event == nil {
				t.Fatalf("Expected response event, actually nil")
			}

			// The TTL will be added again.
			expectedResponseEvent := addTTLToEvent(*tc.returnedEvent)
			if diff := cmp.Diff(expectedResponseEvent.Context.AsV1(), resp.Event.Context.AsV1()); diff != "" {
				t.Errorf("Incorrect response event context (-want +got): %s", diff)
			}
			if diff := cmp.Diff(expectedResponseEvent.Data, resp.Event.Data); diff != "" {
				t.Errorf("Incorrect response event data (-want +got): %s", diff)
			}
		})
	}
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
}

func (h *fakeHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
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
	if h.returnedEvent == nil {
		resp.WriteHeader(http.StatusAccepted)
		return
	}

	c := &cehttp.CodecV1{}
	m, err := c.Encode(context.Background(), *h.returnedEvent)
	if err != nil {
		h.t.Fatalf("Could not encode message: %v", err)
	}
	msg := m.(*cehttp.Message)
	for k, vs := range msg.Header {
		resp.Header().Del(k)
		for _, v := range vs {
			resp.Header().Set(k, v)
		}
	}
	_, err = resp.Write(msg.Body)
	if err != nil {
		h.t.Fatalf("Unable to write body: %v", err)
	}
}

type FakeBrokerNamespaceLister struct {
	client *fake.Clientset
}

func (f *FakeBrokerNamespaceLister) List(selector labels.Selector) (ret []*eventingv1alpha1.Trigger, err error) {
	panic("implement me")
}

func (f *FakeBrokerNamespaceLister) Get(name string) (*eventingv1alpha1.Trigger, error) {
	return f.client.EventingV1alpha1().Triggers(testNS).Get(name, metav1.GetOptions{})
}

func getFakeTriggerLister(initial []runtime.Object) v1alpha1.TriggerNamespaceLister {
	c := fake.NewSimpleClientset(initial...)

	return &FakeBrokerNamespaceLister{
		client: c,
	}
}

func makeTriggerFilterWithDeprecatedSourceAndType(t, s string) *eventingv1alpha1.TriggerFilter {
	return &eventingv1alpha1.TriggerFilter{
		DeprecatedSourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
			Type:   t,
			Source: s,
		},
	}
}

func makeTriggerFilterWithAttributes(t, s string) *eventingv1alpha1.TriggerFilter {
	return &eventingv1alpha1.TriggerFilter{
		Attributes: &eventingv1alpha1.TriggerFilterAttributes{
			"type":   t,
			"source": s,
		},
	}
}

func makeTriggerFilterWithAttributesAndExtension(t, s, e string) *eventingv1alpha1.TriggerFilter {
	return &eventingv1alpha1.TriggerFilter{
		Attributes: &eventingv1alpha1.TriggerFilterAttributes{
			"type":        t,
			"source":      s,
			extensionName: e,
		},
	}
}

func makeTrigger(filter *eventingv1alpha1.TriggerFilter) *eventingv1alpha1.Trigger {
	return &eventingv1alpha1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: eventingv1alpha1.TriggerSpec{
			Filter: filter,
		},
		Status: eventingv1alpha1.TriggerStatus{
			SubscriberURI: &apis.URL{Host: "toBeReplaced"},
		},
	}
}

func makeTriggerWithoutFilter() *eventingv1alpha1.Trigger {
	t := makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", ""))
	t.Spec.Filter = nil
	return t
}

func makeTriggerWithoutSubscriberURI() *eventingv1alpha1.Trigger {
	t := makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", ""))
	t.Status = eventingv1alpha1.TriggerStatus{}
	return t
}

func makeEventWithoutTTL() *cloudevents.Event {
	return &cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: eventType,
			Source: cloudevents.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
		}.AsV1(),
	}
}

func makeEvent() *cloudevents.Event {
	noTTL := makeEventWithoutTTL()
	e := addTTLToEvent(*noTTL)
	return &e
}

func addTTLToEvent(e cloudevents.Event) cloudevents.Event {
	broker.SetTTL(e.Context, 1)
	return e
}

func makeDifferentEvent() *cloudevents.Event {
	return &cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: "some-other-type",
			Source: cloudevents.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
		}.AsV1(),
	}
}

func makeEventWithExtension(extName, extValue string) *cloudevents.Event {
	noTTL := &cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: eventType,
			Source: cloudevents.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
			Extensions: map[string]interface{}{
				extName: extValue,
			},
		}.AsV1(),
	}
	e := addTTLToEvent(*noTTL)
	return &e
}
