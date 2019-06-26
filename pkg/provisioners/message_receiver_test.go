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

package provisioners

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/utils"
	_ "knative.dev/pkg/system/testing"

	"go.uber.org/zap"
)

func TestMessageReceiver_HandleRequest(t *testing.T) {
	testCases := map[string]struct {
		method       string
		host         string
		path         string
		header       http.Header
		body         string
		bodyReader   io.Reader
		expected     int
		receiverFunc func(ChannelReference, *Message) error
	}{
		"non '/' path": {
			path:     "/something",
			expected: http.StatusNotFound,
		},
		"not a POST": {
			method:   http.MethodGet,
			expected: http.StatusMethodNotAllowed,
		},
		"invalid host name": {
			host:     "no-dot",
			expected: http.StatusInternalServerError,
		},
		"unreadable body": {
			bodyReader: &errorReader{},
			expected:   http.StatusInternalServerError,
		},
		"unknown channel error": {
			receiverFunc: func(_ ChannelReference, _ *Message) error {
				return ErrUnknownChannel
			},
			expected: http.StatusNotFound,
		},
		"other receiver function error": {
			receiverFunc: func(_ ChannelReference, _ *Message) error {
				return errors.New("test induced receiver function error")
			},
			expected: http.StatusInternalServerError,
		},
		"headers and body pass through": {
			// The header, body, and host values set here are verified in the receiverFunc. Altering
			// them here will require the same alteration in the receiverFunc.
			header: map[string][]string{
				"not":                       {"passed", "through"},
				"nor":                       {"this-one"},
				"x-requEst-id":              {"1234"},
				"contenT-type":              {"text/json"},
				"knatIve-will-pass-through": {"true", "always"},
				"cE-pass-through":           {"true"},
				"x-B3-pass":                 {"true"},
				"x-ot-pass":                 {"true"},
			},
			body: "message-body",
			host: "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
			receiverFunc: func(r ChannelReference, m *Message) error {
				if r.Namespace != "test-namespace" || r.Name != "test-name" {
					return fmt.Errorf("test receiver func -- bad reference: %v", r)
				}
				if string(m.Payload) != "message-body" {
					return fmt.Errorf("test receiver func -- bad payload: %v", m.Payload)
				}
				expectedHeaders := map[string]string{
					"x-requEst-id": "1234",
					"contenT-type": "text/json",
					// Note that only the first value was passed through, the remaining values were
					// discarded.
					"knatIve-will-pass-through": "true",
					"cE-pass-through":           "true",
					"x-B3-pass":                 "true",
					"x-ot-pass":                 "true",
					"ce-knativehistory":         "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
				}
				if diff := cmp.Diff(expectedHeaders, m.Headers); diff != "" {
					return fmt.Errorf("test receiver func -- bad headers (-want, +got): %s", diff)
				}
				return nil
			},
			expected: http.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			// Default the common things.
			if tc.method == "" {
				tc.method = http.MethodPost
			}
			if tc.path == "" {
				tc.path = "/"
			}
			if tc.host == "" {
				tc.host = "test-channel.test-namespace.svc." + utils.GetClusterDomainName()
			}

			f := tc.receiverFunc
			r, err := NewMessageReceiver(f, zap.NewNop().Sugar())
			if err != nil {
				t.Fatalf("Error creating new message receiver. Error:%s", err)
			}
			h := r.handler()

			body := tc.bodyReader
			if body == nil {
				body = strings.NewReader(tc.body)
			}

			req := httptest.NewRequest(tc.method, tc.path, body)
			req.Host = tc.host
			req.Header = tc.header

			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			if resp.Code != tc.expected {
				t.Fatalf("Unexpected status code. Expected %v. Actual %v", tc.expected, resp.Code)
			}
		})
	}
}

type errorReader struct{}

var _ io.Reader = &errorReader{}

func (*errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("errorReader returns an error")
}
