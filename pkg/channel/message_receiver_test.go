/*
Copyright 2020 The Knative Authors

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

package channel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"go.opencensus.io/trace"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing/pkg/utils"

	"go.uber.org/zap"
)

func TestMessageReceiver_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		method            string
		host              string
		path              string
		additionalHeaders nethttp.Header
		expected          int
		receiverFunc      UnbufferedMessageReceiverFunc
	}{
		"non '/' path": {
			path:     "/something",
			expected: nethttp.StatusNotFound,
		},
		"not a POST": {
			method:   nethttp.MethodGet,
			expected: nethttp.StatusMethodNotAllowed,
		},
		"invalid host name": {
			host:     "no-dot",
			expected: nethttp.StatusInternalServerError,
		},
		"unknown channel error": {
			receiverFunc: func(_ context.Context, c ChannelReference, _ binding.Message, _ []binding.TransformerFactory, _ nethttp.Header) error {
				return &UnknownChannelError{c: c}
			},
			expected: nethttp.StatusNotFound,
		},
		"other receiver function error": {
			receiverFunc: func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.TransformerFactory, _ nethttp.Header) error {
				return errors.New("test induced receiver function error")
			},
			expected: nethttp.StatusInternalServerError,
		},
		"headers and body pass through": {
			// The header, body, and host values set here are verified in the receiverFunc. Altering
			// them here will require the same alteration in the receiverFunc.
			additionalHeaders: map[string][]string{
				"not":                       {"passed", "through"},
				"nor":                       {"this-one"},
				"x-requEst-id":              {"1234"},
				"knatIve-will-pass-through": {"true", "always"},
				// Ce headers won't pass through our header filtering as they should actually be set in the CloudEvent itself,
				// as extensions. The SDK then sets them as as Ce- headers when sending them through HTTP.
				"cE-not-pass-through": {"true"},
				"x-B3-pass":           {"will not pass"},
				"x-ot-pass":           {"will not pass"},
			},
			host: "test-name.test-namespace.svc." + utils.GetClusterDomainName(),
			receiverFunc: func(ctx context.Context, r ChannelReference, m binding.Message, transformers []binding.TransformerFactory, additionalHeaders nethttp.Header) error {
				if r.Namespace != "test-namespace" || r.Name != "test-name" {
					return fmt.Errorf("test receiver func -- bad reference: %v", r)
				}

				e, err := binding.ToEvent(ctx, m, transformers...)
				if err != nil {
					return err
				}

				// Check payload
				var payload string
				err = e.DataAs(&payload)
				if err != nil {
					return err
				}
				if payload != "event-body" {
					return fmt.Errorf("test receiver func -- bad payload: %v", payload)
				}

				// Check headers
				expectedHeaders := make(nethttp.Header)
				expectedHeaders.Add("x-requEst-id", "1234")
				expectedHeaders.Add("knatIve-will-pass-through", "true")
				expectedHeaders.Add("knatIve-will-pass-through", "always")
				if diff := cmp.Diff(expectedHeaders, additionalHeaders); diff != "" {
					return fmt.Errorf("test receiver func -- bad headers (-want, +got): %s", diff)
				}

				// Check history
				if h, ok := e.Extensions()[EventHistory]; !ok {
					return fmt.Errorf("test receiver func -- history not added")
				} else {
					expectedHistory := "test-name.test-namespace.svc." + utils.GetClusterDomainName()
					if h != expectedHistory {
						return fmt.Errorf("test receiver func -- bad history: %v", h)
					}
				}

				// Check traceparent
				if tr, ok := e.Extensions()["traceparent"]; !ok || tr == "" {
					return fmt.Errorf("test receiver func -- trace not added or empty: %s", tr)
				}

				return nil
			},
			expected: nethttp.StatusAccepted,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			// Default the common things.
			if tc.method == "" {
				tc.method = nethttp.MethodPost
			}
			if tc.path == "" {
				tc.path = "/"
			}
			if tc.host == "" {
				tc.host = "test-channel.test-namespace.svc." + utils.GetClusterDomainName()
			}

			f := tc.receiverFunc
			r, err := NewMessageReceiver(f, zap.NewNop())
			if err != nil {
				t.Fatalf("Error creating new event receiver. Error:%s", err)
			}

			event := test.FullEvent()
			err = event.SetData("text/plain", []byte("event-body"))
			if err != nil {
				t.Fatal(err)
			}

			req := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, nil)
			reqCtx, _ := trace.StartSpan(context.TODO(), "bla")
			req = req.WithContext(reqCtx)
			req.Host = tc.host

			err = http.WriteRequest(context.TODO(), binding.ToMessage(&event), req, binding.TransformerFactories{})
			if err != nil {
				t.Fatal(err)
			}

			if tc.additionalHeaders != nil {
				for h, values := range tc.additionalHeaders {
					for _, v := range values {
						req.Header.Add(h, v)
					}
				}
			}

			res := httptest.ResponseRecorder{}

			r.ServeHTTP(&res, req)
			if res.Code != tc.expected {
				t.Fatalf("Unexpected status code. Expected %v. Actual %v", tc.expected, res.Code)
			}
		})
	}
}

func TestMessageReceiver_WrongRequest(t *testing.T) {
	host := "http://test-channel.test-namespace.svc." + utils.GetClusterDomainName() + "/"

	f := func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.TransformerFactory, _ nethttp.Header) error {
		return errors.New("test induced receiver function error")
	}
	r, err := NewMessageReceiver(f, zap.NewNop())
	if err != nil {
		t.Fatalf("Error creating new event receiver. Error:%s", err)
	}

	req := httptest.NewRequest(nethttp.MethodPost, host, bytes.NewReader([]byte("{}")))
	req.Header.Set("content-type", "application/json")

	res := httptest.ResponseRecorder{}

	r.ServeHTTP(&res, req)
	if res.Code != 400 {
		t.Fatalf("Unexpected status code. Expected 400. Actual %v", res.Code)
	}
}

func TestMessageReceiver_UnknownHost(t *testing.T) {
	host := "http://test-channel.test-namespace.svc." + utils.GetClusterDomainName() + "/"

	f := func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.TransformerFactory, _ nethttp.Header) error {
		return errors.New("test induced receiver function error")
	}
	r, err := NewMessageReceiver(f, zap.NewNop(), ResolveMessageChannelFromHostHeader(func(s string) (reference ChannelReference, err error) {
		return ChannelReference{}, UnknownHostError(s)
	}))
	if err != nil {
		t.Fatalf("Error creating new event receiver. Error:%s", err)
	}

	event := test.FullEvent()
	err = event.SetData("text/plain", []byte("event-body"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "http://localhost:8080/", nil)
	req.Host = host

	err = http.WriteRequest(context.TODO(), binding.ToMessage(&event), req, binding.TransformerFactories{})
	if err != nil {
		t.Fatal(err)
	}

	res := httptest.ResponseRecorder{}

	r.ServeHTTP(&res, req)
	if res.Code != 404 {
		t.Fatalf("Unexpected status code. Expected 404. Actual %v", res.Code)
	}
}
