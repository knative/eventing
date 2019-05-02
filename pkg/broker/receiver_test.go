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

package broker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	controllertesting "github.com/knative/eventing/pkg/reconciler/testing"
	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	eventType   = `com.example.someevent`
	eventSource = `/mycontext`

	toBeReplaced = "toBeReplaced"
)

var (
	host      = fmt.Sprintf("%s.%s.triggers.%s", triggerName, testNS, utils.GetClusterDomainName())
	validPath = fmt.Sprintf("/triggers/%s/%s", testNS, triggerName)
)

func init() {
	// Add types to scheme.
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		triggers         []*TriggerBuilder
		mocks            controllertesting.Mocks
		tctx             *cloudevents.HTTPTransportContext
		event            *EventBuilder
		requestFails     bool
		returnedEvent    *EventBuilder
		expectNewToFail  bool
		expectedErr      bool
		expectedDispatch bool
		expectedStatus   int
		expectedHeaders  http.Header
	}{
		"Cannot init": {
			mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test induced error")
					},
				},
			},
			expectNewToFail: true,
		},
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
			triggers: []*TriggerBuilder{
				Trigger(),
			},
			expectedErr: true,
		},
		"Trigger with bad SubscriberURI": {
			triggers: []*TriggerBuilder{
				Trigger().BadSubscriberURI(),
			},
			expectedErr: true,
		},
		"Trigger without a Filter": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI(),
			},
		},
		"No TTL": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", ""),
			},
			event: EventWithoutTTL(),
		},
		"Wrong type": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("some-other-type", ""),
			},
		},
		"Wrong source": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", "some-other-source"),
			},
		},
		"Dispatch failed": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", ""),
			},
			requestFails:     true,
			expectedErr:      true,
			expectedDispatch: true,
		},
		"Dispatch succeeded - SourceAndType Any": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", ""),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - SourceAndType Specific": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType(eventType, eventSource),
			},
			expectedDispatch: true,
		},
		"Attributes type": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("type", eventType),
			},
			expectedDispatch: true,
		},
		"Attributes wrong type": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("type", "some-other-type"),
			},
		},
		"Attributes source": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("source", eventSource),
			},
			expectedDispatch: true,
		},
		"Attributes wrong source": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("source", "some-other-source"),
			},
		},
		"Attributes type and source correct": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("type", eventType, "source", eventSource),
			},
			expectedDispatch: true,
		},
		"Attributes type correct source wrong": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("type", eventType, "source", "some-other-source"),
			},
		},
		"Attributes extension attribute": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("foo", "bar"),
			},
			event:            Event().Extension("foo", "bar"),
			expectedDispatch: true,
		},
		"Attributes dotted extension attribute": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterAttributes("foo.bar", "baz"),
			},
			event:            Event().Extension("foo.bar", "baz"),
			expectedDispatch: true,
		},
		"Expression wrong type": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`ce.type == "some-other-type"`),
			},
		},
		"Expression wrong source": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`ce.source == "some-other-source"`),
			},
		},
		"Expression wrong parsed extensions": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`ext.foo == "baz"`),
			},
			event: Event().Extension("foo", "bar"),
		},
		"Expression wrong parsed data": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`data.baz == "quz"`),
			},
			event: Event().JSONData(`{"baz":"qux"}`),
		},
		"Dispatch succeeded - Expression Any": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression("1 == 1"),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression Specific": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(fmt.Sprintf(`ce.type == "%s" && ce.source == "%s"`, eventType, eventSource)),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression parsed extensions": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`ce.foo == "bar"`),
			},
			event:            Event().Extension("foo", "bar"),
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression parsed data": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`data.baz == "qux"`),
			},
			event:            Event().JSONData(`{"baz":"qux"}`),
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression parsed data integer": {
			triggers: []*TriggerBuilder{
				// TODO Revisit this when CEL can compare floats and ints
				Trigger().SubscriberURI().FilterExpression(`data.baz == 3.0`),
			},
			event:            Event().JSONData(`{"baz":3}`),
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression parsed data float": {
			triggers: []*TriggerBuilder{
				// TODO Revisit this when CEL can compare floats and ints
				Trigger().SubscriberURI().FilterExpression(`data.baz == 3.2`),
			},
			event:            Event().JSONData(`{"baz":3.2}`),
			expectedDispatch: true,
		},
		"Dispatch succeeded - Expression parsed data nested field": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterExpression(`data.foo.bar == "baz"`),
			},
			event:            Event().JSONData(`{"foo":{"bar":"baz"}}`),
			expectedDispatch: true,
		},
		"Returned Cloud Event": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", ""),
			},
			expectedDispatch: true,
			returnedEvent:    Event().Type("some-other-type"),
		},
		"Returned Cloud Event with custom headers": {
			triggers: []*TriggerBuilder{
				Trigger().SubscriberURI().FilterSourceAndType("", ""),
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
			returnedEvent:    Event().Type("some-other-type"),
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
				correctURI = append(correctURI, trig.Build())
			}

			r, err := New(
				zap.NewNop(),
				getClient(correctURI, tc.mocks))
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
				event = Event()
			}
			err = r.serveHTTP(ctx, *event.Build(), resp)

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
			expectedResponseEvent := *tc.returnedEvent.Build()
			if diff := cmp.Diff(expectedResponseEvent.Context.AsV02(), resp.Event.Context.AsV02()); diff != "" {
				t.Errorf("Incorrect response event context (-want +got): %s", diff)
			}
			if diff := cmp.Diff(expectedResponseEvent.Data, resp.Event.Data); diff != "" {
				t.Errorf("Incorrect response event data (-want +got): %s", diff)
			}
		})
	}
}

type fakeHandler struct {
	failRequest     bool
	requestReceived bool
	headers         http.Header
	returnedEvent   *EventBuilder
	t               *testing.T
}

func (h *fakeHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	h.requestReceived = true

	for n, v := range h.headers {
		if strings.Contains(strings.ToLower(n), strings.ToLower(V02TTLAttribute)) {
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

	c := &cehttp.CodecV02{}
	m, err := c.Encode(*h.returnedEvent.Build())
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

func getClient(initial []runtime.Object, mocks controllertesting.Mocks) *controllertesting.MockClient {
	innerClient := fake.NewFakeClient(initial...)
	return controllertesting.NewMockClient(innerClient, mocks)
}

type TriggerBuilder struct {
	*eventingv1alpha1.Trigger
}

var _ controllertesting.Buildable = &TriggerBuilder{}

func Trigger() *TriggerBuilder {
	trigger := &eventingv1alpha1.Trigger{
		TypeMeta: v1.TypeMeta{
			APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Trigger",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
		},
	}
	return &TriggerBuilder{
		Trigger: trigger,
	}
}

func (b *TriggerBuilder) Build() runtime.Object {
	return b.Trigger
}

func (b *TriggerBuilder) FilterSourceAndType(t, s string) *TriggerBuilder {
	b.Spec.Filter = &eventingv1alpha1.TriggerFilter{
		DeprecatedSourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
			Type:   t,
			Source: s,
		},
	}
	return b
}

func (b *TriggerBuilder) FilterAttributes(args ...string) *TriggerBuilder {
	if len(args)%2 != 0 {
		panic(fmt.Errorf("args should have even number of elements but has %d", len(args)))
	}
	attrs := eventingv1alpha1.TriggerFilterAttributes{}
	for i := 0; i < len(args); i = i + 2 {
		attrs[args[i]] = args[i+1]
	}
	b.Spec.Filter = &eventingv1alpha1.TriggerFilter{
		Attributes: &attrs,
	}
	return b
}

func (b *TriggerBuilder) FilterExpression(expr string) *TriggerBuilder {
	tfe := eventingv1alpha1.TriggerFilterExpression(expr)
	b.Spec.Filter = &eventingv1alpha1.TriggerFilter{
		Expression: &tfe,
	}
	return b
}

func (b *TriggerBuilder) SubscriberURI() *TriggerBuilder {
	b.Status = eventingv1alpha1.TriggerStatus{
		SubscriberURI: toBeReplaced,
	}
	return b
}

func (b *TriggerBuilder) BadSubscriberURI() *TriggerBuilder {
	// This should fail url.Parse(). It was taken from the unit tests for
	// url.Parse(), it violates rfc3986 3.2.3, namely that the port must be
	// digits.
	b.Status = eventingv1alpha1.TriggerStatus{
		SubscriberURI: "http://[::1]:namedport",
	}
	return b
}

type EventBuilder struct {
	*cloudevents.Event
}

func (b *EventBuilder) Build() *cloudevents.Event {
	return b.Event
}

func EventWithoutTTL() *EventBuilder {
	event := &cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: eventType,
			Source: cloudevents.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
		}.AsV02(),
	}
	return &EventBuilder{
		Event: event,
	}
}

func Event() *EventBuilder {
	return EventWithoutTTL().TTL()
}

func (b *EventBuilder) TTL() *EventBuilder {
	b.Context, _ = SetTTL(b.Context, 1)
	return b
}

func (b *EventBuilder) Type(t string) *EventBuilder {
	ctx := b.Context.AsV02()
	ctx.Type = t
	b.Context = ctx
	return b
}

func (b *EventBuilder) DataContentType(t string) *EventBuilder {
	ctx := b.Context.AsV02()
	ctx.ContentType = &t
	b.Context = ctx
	return b
}

func (b *EventBuilder) Extension(k string, v interface{}) *EventBuilder {
	ctx := b.Context.AsV02()
	ctx.SetExtension(k, v)
	b.Context = ctx
	return b
}

func (b *EventBuilder) JSONData(d string) *EventBuilder {
	b = b.DataContentType("application/json")
	b.Data = []byte(d)
	return b
}
