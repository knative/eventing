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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func init() {
	// Add types to scheme.
	eventingv1alpha1.AddToScheme(scheme.Scheme)
}

func TestReceiver(t *testing.T) {
	testCases := map[string]struct {
		initialState     []runtime.Object
		requestFails     bool
		expectedErr      bool
		expectedDispatch bool
	}{
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
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			mr := New(
				zap.NewNop(),
				fake.NewFakeClient(tc.initialState...))
			fh := &fakeHTTPDoer{
				failRequest: tc.requestFails,
			}
			mr.httpClient = fh

			resp := httptest.NewRecorder()
			mr.ServeHTTP(resp, makeRequest())
			if tc.expectedErr {
				if resp.Result().StatusCode >= 200 && resp.Result().StatusCode < 300 {
					t.Errorf("Expected an error. Actual: %v", resp.Result())
				}
			} else {
				if resp.Result().StatusCode < 200 || resp.Result().StatusCode >= 300 {
					t.Errorf("Expected success. Actual: %v", resp.Result())
				}
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

func makeRequest() *http.Request {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`<much wow="xml"/>`))
	req.Host = fmt.Sprintf("%s.%s.triggers.%s", triggerName, testNS, utils.GetClusterDomainName())

	eventAttributes := map[string]string{
		"CE-CloudEventsVersion": `0.1`,
		"CE-EventType":          eventType,
		"CE-EventTypeVersion":   `1.0`,
		"CE-Source":             eventSource,
		"CE-EventID":            `A234-1234-1234`,
		"CE-EventTime":          `2018-04-05T17:31:00Z`,
		"Content-Type":          "application/xml",
	}
	for k, v := range eventAttributes {
		req.Header.Set(k, v)
	}
	return req
}
