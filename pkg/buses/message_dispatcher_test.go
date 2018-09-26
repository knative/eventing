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
		message         *Message
		expectedHeaders http.Header
	}{
		"filter unwanted headers": {
			message: &Message{
				Headers: map[string]string{
					"do-not-forward": "header",
				},
				Payload: []byte("{}"),
			},
			expectedHeaders: map[string][]string{},
		},
		"multiple forward prefixes": {
			message: &Message{
				Headers: map[string]string{
					"x-request-id": "id123",
					"knative-1":    "knative-1-value",
					"knative-2":    "knative-2-value",
					"ce-abc":       "ce-abc-value",
				},
				Payload: []byte("{}"),
			},
			expectedHeaders: map[string][]string{
				"x-request-id": {"id123"},
				"knative-1":    {"knative-1-value"},
				"knative-2":    {"knative-2-value"},
				"ce-abc":       {"ce-abc-value"},
			},
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			md := NewMessageDispatcher(zap.NewNop().Sugar())
			fc := &fakeHttpClient{}
			md.httpClient = fc
			err := md.DispatchMessage(tc.message, "destination", "", DispatchDefaults{})
			if err != nil {
				t.Errorf("Unexpected error dispatching message: %v", err)
			}
			if diff := headerDiff(tc.expectedHeaders, fc.requestHeaders); diff != "" {
				t.Errorf("Unexpected request headers (-wanted, +got): %v", diff)
			}
		})
	}
}

type fakeHttpClient struct {
	requestHeaders http.Header
}

var _ httpDoer = &fakeHttpClient{}

func (f *fakeHttpClient) Do(r *http.Request) (*http.Response, error) {
	f.requestHeaders = r.Header
	return &http.Response{
		StatusCode: http.StatusAccepted,
		Body:       ioutil.NopCloser(bytes.NewBufferString("body")),
	}, nil
}

func headerDiff(expected http.Header, actual http.Header) string {
	// HTTP header names are case-insensitive, so normalize them to lower case for comparison.
	for _, headers := range []http.Header{expected, actual} {
		for n, v := range headers {
			delete(headers, n)
			headers[strings.ToLower(n)] = v
		}
	}
	return cmp.Diff(expected, actual)
}
