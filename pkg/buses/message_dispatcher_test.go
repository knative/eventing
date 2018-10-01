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
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

var (
	// Headers that are added to the response, but we don't want to check in our assertions.
	unimportantHeaders = map[string]struct{}{
		"accept-encoding": {},
		"content-length":  {},
		"content-type":    {},
		"user-agent":      {},
	}
)

func TestDispatchMessage(t *testing.T) {
	testCases := map[string]struct {
		sendToDestination    bool
		sendToReply          bool
		message              *Message
		fakeResponse         *http.Response
		expectedErr          bool
		expectedDestRequest  *requestValidation
		expectedReplyRequest *requestValidation
	}{
		"destination - only": {
			sendToDestination: true,
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
			sendToDestination: true,
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
			sendToReply: true,
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
			sendToReply: true,
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
			sendToDestination: true,
			sendToReply:       true,
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
			sendToDestination: true,
			sendToReply:       true,
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
			sendToDestination: true,
			sendToReply:       true,
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
			destHandler := &fakeHandler{
				t:        t,
				response: tc.fakeResponse,
				requests: make([]requestValidation, 0),
			}
			destServer := httptest.NewServer(destHandler)
			defer destServer.Close()
			replyHandler := &fakeHandler{
				t:        t,
				response: tc.fakeResponse,
				requests: make([]requestValidation, 0),
			}
			replyServer := httptest.NewServer(replyHandler)
			defer replyServer.Close()

			md := NewMessageDispatcher(zap.NewNop().Sugar())
			err := md.DispatchMessage(tc.message,
				getDomain(t, tc.sendToDestination, destServer.URL),
				getDomain(t, tc.sendToReply, replyServer.URL),
				DispatchDefaults{})
			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error from DispatchRequest. Expected %v. Actual: %v", tc.expectedErr, err)
			}
			if tc.expectedDestRequest != nil {
				rv := destHandler.popRequest(t)
				assertEquality(t, destServer.URL, *tc.expectedDestRequest, rv)
			}
			if tc.expectedReplyRequest != nil {
				rv := replyHandler.popRequest(t)
				assertEquality(t, replyServer.URL, *tc.expectedReplyRequest, rv)
			}
			if len(destHandler.requests) != 0 {
				t.Errorf("Unexpected destination requests: %+v", destHandler.requests)
			}
			if len(replyHandler.requests) != 0 {
				t.Errorf("Unexpected reply requests: %+v", replyHandler.requests)
			}
		})
	}
}

func getDomain(t *testing.T, shouldSend bool, serverURL string) string {
	if shouldSend {
		server, err := url.Parse(serverURL)
		if err != nil {
			t.Errorf("Bad serverURL: %q", serverURL)
		}
		return server.Host
	}
	return ""
}

type requestValidation struct {
	Host    string
	Headers http.Header
	Body    string
}

type fakeHandler struct {
	t        *testing.T
	response *http.Response
	requests []requestValidation
}

func (f *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Make a copy of the request.
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		f.t.Error("Failed to read the request body")
	}
	f.requests = append(f.requests, requestValidation{
		Host:    r.Host,
		Headers: r.Header,
		Body:    string(body),
	})

	// Write the response.
	if f.response != nil {
		for h, vs := range f.response.Header {
			for _, v := range vs {
				w.Header().Add(h, v)
			}
		}
		w.WriteHeader(f.response.StatusCode)
		var buf bytes.Buffer
		buf.ReadFrom(f.response.Body)
		w.Write(buf.Bytes())
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(""))
	}
}

func (f *fakeHandler) popRequest(t *testing.T) requestValidation {
	if len(f.requests) == 0 {
		t.Error("Unable to pop request")
	}
	rv := f.requests[0]
	f.requests = f.requests[1:]
	return rv
}

func assertEquality(t *testing.T, replacementURL string, expected, actual requestValidation) {
	server, err := url.Parse(replacementURL)
	if err != nil {
		t.Errorf("Bad replacement URL: %q", replacementURL)
	}
	expected.Host = server.Host
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
			ln := strings.ToLower(n)
			if _, present := unimportantHeaders[ln]; !present {
				headers[ln] = v
			}
		}
	}
}
