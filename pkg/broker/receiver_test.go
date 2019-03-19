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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"

	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNS      = "test-namespace"
	triggerName = "test-trigger"
	eventType   = `com.example.someevent`
	eventSource = `/mycontext`
)

var (
	host = fmt.Sprintf("%s.%s.triggers.%s", triggerName, testNS, utils.GetClusterDomainName())
)

func init() {
	// Add types to scheme.
	_ = eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		triggers         []*eventingv1alpha1.Trigger
		tctx             *cehttp.TransportContext
		requestFails     bool
		returnedEvent    *cloudevents.Event
		expectedErr      bool
		expectedDispatch bool
		expectedStatus   int
	}{
		"Not POST": {
			tctx: &cehttp.TransportContext{
				Method: "GET",
				Host:   host,
				URI:    "/",
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		"Other path": {
			tctx: &cehttp.TransportContext{
				Method: "POST",
				Host:   host,
				URI:    "/someotherEndpoint",
			},
			expectedStatus: http.StatusNotFound,
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
		"Trigger without a Filter": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTriggerWithoutFilter(),
			},
		},
		"Wrong type": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger("some-other-type", "Any"),
			},
		},
		"Wrong source": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger("Any", "some-other-source"),
			},
		},
		"Dispatch failed": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger("Any", "Any"),
			},
			requestFails:     true,
			expectedErr:      true,
			expectedDispatch: true,
		},
		"Dispatch succeeded - Any": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger("Any", "Any"),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Specific": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger(eventType, eventSource),
			},
			expectedDispatch: true,
		},
		"Returned Cloud Event": {
			triggers: []*eventingv1alpha1.Trigger{
				makeTrigger("Any", "Any"),
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
			}
			s := httptest.NewServer(&fh)
			defer s.Client()

			// Replace the SubscriberURI to point at our fake server.
			correctURI := make([]runtime.Object, 0, len(tc.triggers))
			for _, trig := range tc.triggers {
				if trig.Status.SubscriberURI != "" {
					trig.Status.SubscriberURI = s.URL
				}
				correctURI = append(correctURI, trig)
			}

			r, err := New(
				zap.NewNop(),
				fake.NewFakeClient(correctURI...))
			if err != nil {
				t.Fatalf("Unable to create receiver: %v", err)
			}

			tctx := tc.tctx
			if tctx == nil {
				tctx = &cehttp.TransportContext{
					Method: http.MethodPost,
					Host:   host,
					URI:    "/",
				}
			}
			ctx := cehttp.WithTransportContext(context.Background(), *tctx)
			resp := &cloudevents.EventResponse{}
			err = r.serveHTTP(ctx, makeEvent(), resp)

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
					t.Errorf("Unexpected response event: %v", resp.Event)
				}
				return
			}
			if diff := cmp.Diff(tc.returnedEvent.Context.AsV02(), resp.Event.Context.AsV02()); diff != "" {
				t.Errorf("Incorrect response event context (-want +got): %s", diff)
			}
			if diff := cmp.Diff(tc.returnedEvent.Data, resp.Event.Data); diff != "" {
				t.Errorf("Incorrect response event data (-want +got): %s", diff)
			}
		})
	}
}

type fakeHandler struct {
	failRequest     bool
	requestReceived bool
	returnedEvent   *cloudevents.Event
	t               testing.T
}

func (h *fakeHandler) ServeHTTP(resp http.ResponseWriter, _ *http.Request) {
	h.requestReceived = true
	if h.failRequest {
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	if h.returnedEvent == nil {
		resp.WriteHeader(http.StatusAccepted)
		return
	}

	c := &cehttp.CodecV02{}
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

func makeTrigger(t, s string) *eventingv1alpha1.Trigger {
	return &eventingv1alpha1.Trigger{
		TypeMeta: v1.TypeMeta{
			APIVersion: "eventing.knative.dev/v1alpha1",
			Kind:       "Trigger",
		},
		ObjectMeta: v1.ObjectMeta{
			Namespace: testNS,
			Name:      triggerName,
		},
		Spec: eventingv1alpha1.TriggerSpec{
			Filter: &eventingv1alpha1.TriggerFilter{
				SourceAndType: &eventingv1alpha1.TriggerFilterSourceAndType{
					Type:   t,
					Source: s,
				},
			},
		},
		Status: eventingv1alpha1.TriggerStatus{
			SubscriberURI: "subscriberURI",
		},
	}
}

func makeTriggerWithoutFilter() *eventingv1alpha1.Trigger {
	t := makeTrigger("Any", "Any")
	t.Spec.Filter = nil
	return t
}

func makeTriggerWithoutSubscriberURI() *eventingv1alpha1.Trigger {
	t := makeTrigger("Any", "Any")
	t.Status = eventingv1alpha1.TriggerStatus{}
	return t
}

func makeEvent() cloudevents.Event {
	return cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: eventType,
			Source: types.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
		},
	}
}

func makeDifferentEvent() *cloudevents.Event {
	return &cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: "some-other-type",
			Source: types.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
			ContentType: cloudevents.StringOfApplicationJSON(),
		},
	}
}
