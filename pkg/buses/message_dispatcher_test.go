/*
Copyright 2018 The Knative Authors

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

package buses

import (
	"bytes"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestDispatchMessage(t *testing.T) {
	testCases := map[string]struct {
		destination          string
		replyTo              string
		message              *Message
		fakeResponse         *http.Response
		expectedErr          bool
		expectedDestRequest  *requestValidation
		expectedReplyRequest *requestValidation
	}{
		"destination - only": {
			destination: "test-destination-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("destination"),
			},
			expectedDestRequest: &requestValidation{
				Url: "http://test-destination-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "destination",
			},
		},
		"destination - only -- error": {
			destination: "test-destination-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("destination"),
			},
			expectedDestRequest: &requestValidation{
				Url: "http://test-destination-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "destination",
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"reply - only": {
			replyTo: "test-reply-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("replyTo"),
			},
			expectedReplyRequest: &requestValidation{
				Url: "http://test-reply-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "replyTo",
			},
		},
		"reply - only -- error": {
			replyTo: "test-reply-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("replyTo"),
			},
			expectedReplyRequest: &requestValidation{
				Url: "http://test-reply-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "replyTo",
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"destination and reply - dest returns bad status code": {
			destination: "test-destination-svc.test-namespace.svc.cluster.local",
			replyTo:     "test-reply-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("destination"),
			},
			expectedDestRequest: &requestValidation{
				Url: "http://test-destination-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "destination",
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedErr: true,
		},
		"destination and reply - dest returns empty body": {
			destination: "test-destination-svc.test-namespace.svc.cluster.local",
			replyTo:     "test-reply-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("destination"),
			},
			expectedDestRequest: &requestValidation{
				Url: "http://test-destination-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "destination",
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {"new-ce-abc-value"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			},
		},
		"destination and reply": {
			destination: "test-destination-svc.test-namespace.svc.cluster.local",
			replyTo:     "test-reply-svc.test-namespace.svc.cluster.local",
			message: &Message{
				Headers: map[string]string{
					// do-not-forward should not get forwarded.
					"do-not-forward": "header",
					"x-request-id":   "id123",
					"knative-1":      "knative-1-value",
					"knative-2":      "knative-2-value",
					"ce-abc":         "ce-abc-value",
				},
				Payload: []byte("destination"),
			},
			expectedDestRequest: &requestValidation{
				Url: "http://test-destination-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"id123"},
					"knative-1":    {"knative-1-value"},
					"knative-2":    {"knative-2-value"},
					"ce-abc":       {"ce-abc-value"},
				},
				Body: "destination",
			},
			fakeResponse: &http.Response{
				StatusCode: http.StatusAccepted,
				Header: map[string][]string{
					"do-not-passthrough": {"no"},
					"x-request-id":       {"altered-id"},
					"knative-1":          {"new-knative-1-value"},
					"ce-abc":             {"new-ce-abc-value"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("destination-response")),
			},
			expectedReplyRequest: &requestValidation{
				Url: "http://test-reply-svc.test-namespace.svc.cluster.local/",
				Headers: map[string][]string{
					"x-request-id": {"altered-id"},
					"knative-1":    {"new-knative-1-value"},
					"ce-abc":       {"new-ce-abc-value"},
				},
				Body: "destination-response",
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			md := NewMessageDispatcher(zap.NewNop().Sugar())
			fc := &fakeHttpClient{
				response: tc.fakeResponse,
			}
			md.httpClient = fc
			err := md.DispatchMessage(tc.message, tc.destination, tc.replyTo, DispatchDefaults{})
			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error from DispatchRequest. Expected %v. Actual: %v", tc.expectedErr, err)
			}
			if tc.expectedDestRequest != nil {
				rv := fc.popRequest(t)
				assertEquality(t, *tc.expectedDestRequest, rv)
			}
			if tc.expectedReplyRequest != nil {
				rv := fc.popRequest(t)
				assertEquality(t, *tc.expectedReplyRequest, rv)
			}
			if len(fc.requests) != 0 {
				t.Errorf("Unexpected requests: %+v", fc.requests)
			}
		})
	}
}

type requestValidation struct {
	Url     string
	Headers http.Header
	Body    string
}

type fakeHttpClient struct {
	t        *testing.T
	response *http.Response
	requests []requestValidation
}

var _ httpDoer = &fakeHttpClient{}

func (f *fakeHttpClient) Do(r *http.Request) (*http.Response, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		f.t.Error("Failed to read the request body")
	}
	f.requests = append(f.requests, requestValidation{
		Url:     r.URL.String(),
		Headers: r.Header,
		Body:    string(body),
	})
	if f.response != nil {
		return f.response, nil
	}
	return &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       ioutil.NopCloser(bytes.NewBufferString("body")),
	}, nil
}

func (f *fakeHttpClient) popRequest(t *testing.T) requestValidation {
	if len(f.requests) == 0 {
		t.Error("Unable to pop request")
	}
	rv := f.requests[0]
	f.requests = f.requests[1:]
	return rv
}

func assertEquality(t *testing.T, expected, actual requestValidation) {
	canonicalizeHeaders(expected, actual)
	if diff := cmp.Diff(expected, actual); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}
}

func canonicalizeHeaders(rvs ...requestValidation) {
	// HTTP header names are case-insensitive, so normalize them to lower case for comparison.
	for _, rv := range rvs {
		headers := rv.Headers
		for n, v := range headers {
			delete(headers, n)
			headers[strings.ToLower(n)] = v
		}
	}
}
