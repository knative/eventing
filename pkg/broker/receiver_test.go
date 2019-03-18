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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"

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
		initialState     []runtime.Object
		tctx             cehttp.TransportContext
		requestFails     bool
		returnedEvent    cloudevents.Event
		expectedErr      bool
		expectedDispatch bool
		expectedStatus   int
	}{
		"Not POST": {
			tctx: cehttp.TransportContext{
				Method: "GET",
				Host:   host,
				URI:    "/",
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		"Other path": {
			tctx: cehttp.TransportContext{
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
			initialState: []runtime.Object{
				makeTriggerWithoutSubscriberURI(),
			},
			expectedErr: true,
		},
		"Trigger without a Filter": {
			initialState: []runtime.Object{
				makeTriggerWithoutFilter(),
			},
		},
		"Wrong type": {
			initialState: []runtime.Object{
				makeTrigger("some-other-type", "Any"),
			},
		},
		"Wrong source": {
			initialState: []runtime.Object{
				makeTrigger("Any", "some-other-source"),
			},
		},
		"Dispatch failed": {
			initialState: []runtime.Object{
				makeTrigger("Any", "Any"),
			},
			requestFails:     true,
			expectedErr:      true,
			expectedDispatch: true,
		},
		"Dispatch succeeded - Any": {
			initialState: []runtime.Object{
				makeTrigger("Any", "Any"),
			},
			expectedDispatch: true,
		},
		"Dispatch succeeded - Specific": {
			initialState: []runtime.Object{
				makeTrigger(eventType, eventSource),
			},
			expectedDispatch: true,
		},
		"Returned Cloud Event": {
			returnedEvent: makeDifferentEvent(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			r, err := New(
				zap.NewNop(),
				fake.NewFakeClient(tc.initialState...))
			if err != nil {
				t.Fatalf("Unable to create receiver: %v", err)
			}

			// TODO either use a fake client here or make a fake HTTP server for the test. Either
			// way, update the tc.expectedDispatch test to ensure the request was sent or not.

			r.ceHttp.Client = fakeClient{}

			ctx := context.Background()
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
			if !equality.Semantic.DeepEqual(tc.returnedEvent, resp.Event) {
				t.Errorf("Unexpected response event. Expected '%v'. Actual '%v'", tc.returnedEvent, resp.Event)
			}

			if tc.expectedDispatch != fh.requestReceived {
				t.Errorf("Incorrect dispatch. Expected %v, Actual %v", tc.expectedDispatch, fh.requestReceived)
			}
		})
	}
}

type fakeHTTPDoer struct {
	failRequest     bool
	requestReceived bool
}

func (h *fakeHTTPDoer) Do(_ *http.Request) (*http.Response, error) {
	h.requestReceived = true
	sc := http.StatusOK
	if h.failRequest {
		sc = http.StatusBadRequest
	}
	return &http.Response{
		StatusCode: sc,
		Body:       ioutil.NopCloser(bytes.NewBufferString("")),
	}, nil
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
		},
	}
}

func makeDifferentEvent() cloudevents.Event {
	return cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type: "some-other-type",
			Source: types.URLRef{
				URL: url.URL{
					Path: eventSource,
				},
			},
		},
	}
}
