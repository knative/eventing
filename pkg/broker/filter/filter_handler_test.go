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

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

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

	toBeReplaced = "toBeReplaced"
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
		triggers         []*eventingv1alpha1.Trigger
		tctx             *cloudevents.HTTPTransportContext
		event            *cloudevents.Event
		requestFails     bool
		returnedEvent    *cloudevents.Event
		expectNewToFail  bool
		expectedErr      bool
		expectedDispatch bool
		expectedStatus   int
		expectedHeaders  http.Header
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
			expectedErr: true,
		},
		"Trigger with bad SubscriberURI": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTriggerWithBadSubscriberURI(),
			},
			expectedErr: true,
		},
		"Trigger without a Filter": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTriggerWithoutFilter(),
			},
			expectedDispatch: true,
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
		},
		"Wrong type with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("some-other-type", "")),
			},
		},
		"Wrong source": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "some-other-source")),
			},
		},
		"Wrong source with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
		},
		"Wrong extension": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "some-other-source")),
			},
		},
		"Dispatch failed": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			requestFails:     true,
			expectedErr:      true,
			expectedDispatch: true,
		},
		"Dispatch succeeded - Any": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Any with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes("", "")),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Specific": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType(eventType, eventSource)),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Specific with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributes(eventType, eventSource)),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Extension with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, extensionValue)),
			},
			event:            makeEventWithExtension(),
			expectedDispatch: true,
		},
		"Dispatch failed - Extension with attribs": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithAttributesAndExtension(eventType, eventSource, "some-other-extension-value")),
			},
			event: makeEventWithExtension(),
		},
		"Returned Cloud Event": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", "")),
			},
			expectedDispatch: true,
			returnedEvent:    makeDifferentEvent(),
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
					// X-Request-Id will pass as an exact header match.
					"X-Request-Id": []string{"123"},
					// b3 will pass as an exact header match.
					"B3": []string{"0"},
					// X-B3-Foo will pass as a prefix match.
					"X-B3-Foo": []string{"abc"},
					// Knative-Foo will pass as a prefix match.
					"Knative-Foo": []string{"baz", "qux"},
					// X-Ot-Foo will pass as a prefix match.
					"X-Ot-Foo": []string{"haden"},
				},
			},
			expectedHeaders: http.Header{
				// X-Request-Id will pass as an exact header match.
				"X-Request-Id": []string{"123"},
				// b3 will pass as an exact header match.
				"B3": []string{"0"},
				// X-B3-Foo will pass as a prefix match.
				"X-B3-Foo": []string{"abc"},
				// Knative-Foo will pass as a prefix match.
				"Knative-Foo": []string{"baz", "qux"},
				// X-Ot-Foo will pass as a prefix match.
				"X-Ot-Foo": []string{"haden"},
			},
			expectedDispatch: true,
			returnedEvent:    makeDifferentEvent(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			fh := fakeHandler{
				failRequest:   tc.requestFails,
				returnedEvent: tc.returnedEvent,
				headers:       tc.expectedHeaders,
				t:             t,
			}
			s := httptest.NewServer(&fh)
			defer s.Client()

			// Replace the SubscriberURI to point at our fake server.
			correctURI := make([]runtime.Object, 0, len(tc.triggers))
			for _, trig := range tc.triggers {
				if trig.Status.SubscriberURI == toBeReplaced {
					trig.Status.SubscriberURI = s.URL
				}
				correctURI = append(correctURI, trig)
			}

			r, err := NewHandler(
				zap.NewNop(),
				getFakeTriggerLister(correctURI),
				&mockReporter{})
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
			if diff := cmp.Diff(expectedResponseEvent.Context.AsV03(), resp.Event.Context.AsV03()); diff != "" {
				t.Errorf("Incorrect response event context (-want +got): %s", diff)
			}
			if diff := cmp.Diff(expectedResponseEvent.Data, resp.Event.Data); diff != "" {
				t.Errorf("Incorrect response event data (-want +got): %s", diff)
			}
		})
	}
}

type mockReporter struct{}

func (r *mockReporter) ReportEventCount(args *ReportArgs, err error) error {
	return nil
}

func (r *mockReporter) ReportDispatchTime(args *ReportArgs, err error, d time.Duration) error {
	return nil
}

func (r *mockReporter) ReportFilterTime(args *ReportArgs, filterResult FilterResult, d time.Duration) error {
	return nil
}

func (r *mockReporter) ReportEventDeliveryTime(args *ReportArgs, err error, d time.Duration) error {
	return nil
}

type fakeHandler struct {
	failRequest     bool
	requestReceived bool
	headers         http.Header
	returnedEvent   *cloudevents.Event
	t               *testing.T
}

func (h *fakeHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	h.requestReceived = true

	for n, v := range h.headers {
		if strings.Contains(strings.ToLower(n), strings.ToLower(broker.V03TTLAttribute)) {
			h.t.Errorf("Broker TTL should not be seen by the subscriber: %s", n)
		}
		if diff := cmp.Diff(v, req.Header[n]); diff != "" {
			h.t.Errorf("Incorrect request header '%s' (-want +got): %s", n, diff)
		}
	}

	if h.failRequest {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if h.returnedEvent == nil {
		resp.WriteHeader(http.StatusAccepted)
		return
	}

	c := &cehttp.CodecV03{}
	m, err := c.Encode(*h.returnedEvent)
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
		TypeMeta: v1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Trigger",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
			UID:       triggerUID,
		},
		Spec: eventingv1alpha1.TriggerSpec{
			Filter: filter,
		},
		Status: eventingv1alpha1.TriggerStatus{
			SubscriberURI: "toBeReplaced",
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

func makeTriggerWithBadSubscriberURI() *eventingv1alpha1.Trigger {
	t := makeTrigger(makeTriggerFilterWithDeprecatedSourceAndType("", ""))
	// This should fail url.Parse(). It was taken from the unit tests for url.Parse(), it violates
	// rfc3986 3.2.3, namely that the port must be digits.
	t.Status.SubscriberURI = "http://[::1]:namedport"
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
		}.AsV03(),
	}
}

func makeEvent() *cloudevents.Event {
	noTTL := makeEventWithoutTTL()
	e := addTTLToEvent(*noTTL)
	return &e
}

func addTTLToEvent(e cloudevents.Event) cloudevents.Event {
	e.Context, _ = broker.SetTTL(e.Context, 1)
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
		}.AsV03(),
	}
}

func makeEventWithExtension() *cloudevents.Event {
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
				extensionName: extensionValue,
			},
		}.AsV03(),
	}
	e := addTTLToEvent(*noTTL)
	return &e
}
