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

	"knative.dev/pkg/network"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"
	_ "knative.dev/pkg/system/testing"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/pkg/tracing"
	tracingconfig "knative.dev/pkg/tracing/config"

	"knative.dev/eventing/pkg/kncloudevents"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
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
			receiverFunc: func(_ context.Context, c ChannelReference, _ binding.Message, _ []binding.Transformer, _ nethttp.Header) error {
				return &UnknownChannelError{Channel: c}
			},
			expected: nethttp.StatusNotFound,
		},
		"other receiver function error": {
			receiverFunc: func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.Transformer, _ nethttp.Header) error {
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
			},
			host: "test-name.test-namespace.svc." + network.GetClusterDomainName(),
			receiverFunc: func(ctx context.Context, r ChannelReference, m binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
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

				return nil
			},
			expected: nethttp.StatusAccepted,
		},
	}
	reporter := NewStatsReporter("testcontainer", "testpod")
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
				tc.host = "test-channel.test-namespace.svc." + network.GetClusterDomainName()
			}

			f := tc.receiverFunc
			r, err := NewMessageReceiver(f, zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())), reporter)
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

			err = http.WriteRequest(context.TODO(), binding.ToMessage(&event), req)
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

func TestMessageReceiver_ServerStart_trace_propagation(t *testing.T) {
	want := test.ConvertEventExtensionsToString(t, test.FullEvent())

	done := make(chan struct{}, 1)

	receiverFunc := func(ctx context.Context, r ChannelReference, m binding.Message, transformers []binding.Transformer, additionalHeaders nethttp.Header) error {
		if r.Namespace != "test-namespace" || r.Name != "test-name" {
			return fmt.Errorf("test receiver func -- bad reference: %v", r)
		}

		if span := trace.FromContext(ctx); span == nil {
			return errors.New("missing span")
		}

		done <- struct{}{}

		return nil
	}

	// Default the common things.
	method := nethttp.MethodPost
	host := "test-name.test-namespace.svc." + network.GetClusterDomainName()

	reporter := NewStatsReporter("testcontainer", "testpod")
	logger, _ := zap.NewDevelopment()

	r, err := NewMessageReceiver(receiverFunc, logger, reporter)
	if err != nil {
		t.Fatalf("Error creating new event receiver. Error:%s", err)
	}

	server := httptest.NewServer(kncloudevents.CreateHandler(r))
	defer server.Close()

	require.NoError(t, tracing.SetupStaticPublishing(logger.Sugar(), "localhost", &tracingconfig.Config{
		Backend:        tracingconfig.Zipkin,
		Debug:          true,
		SampleRate:     1.0,
		ZipkinEndpoint: "http://zipkin.zipkin.svc.cluster.local:9411/api/v2/spans",
	}))

	p, err := cloudevents.NewHTTP(
		http.WithTarget(server.URL),
		http.WithMethod(method),
		cloudevents.WithRoundTripper(&ochttp.Transport{
			Propagation: tracecontextb3.TraceContextEgress,
		}))
	require.NoError(t, err)
	p.RequestTemplate.Host = host

	client, err := cloudevents.NewClient(p, cloudevents.WithTracePropagation)
	require.NoError(t, err)

	res := client.Send(context.Background(), want)
	require.True(t, cloudevents.IsACK(res))
	var httpResult *http.Result
	require.True(t, cloudevents.ResultAs(res, &httpResult))
	require.Equal(t, 202, httpResult.StatusCode)

	<-done
}

func TestMessageReceiver_WrongRequest(t *testing.T) {
	reporter := NewStatsReporter("testcontainer", "testpod")
	host := "http://test-channel.test-namespace.svc." + network.GetClusterDomainName() + "/"

	f := func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.Transformer, _ nethttp.Header) error {
		return errors.New("test induced receiver function error")
	}
	r, err := NewMessageReceiver(f, zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())), reporter)
	if err != nil {
		t.Fatalf("Error creating new event receiver. Error:%s", err)
	}

	req := httptest.NewRequest(nethttp.MethodPost, host, bytes.NewReader([]byte("{}")))
	req.Header.Set("content-type", "application/json")

	res := httptest.ResponseRecorder{}

	r.ServeHTTP(&res, req)
	if res.Code != 400 {
		t.Fatal("Unexpected status code. Expected 400. Actual", res.Code)
	}
}

func TestMessageReceiver_UnknownHost(t *testing.T) {
	host := "http://test-channel.test-namespace.svc." + network.GetClusterDomainName() + "/"
	reporter := NewStatsReporter("testcontainer", "testpod")

	f := func(_ context.Context, _ ChannelReference, _ binding.Message, _ []binding.Transformer, _ nethttp.Header) error {
		return errors.New("test induced receiver function error")
	}
	r, err := NewMessageReceiver(
		f,
		zaptest.NewLogger(t, zaptest.WrapOptions(zap.AddCaller())),
		reporter,
		ResolveMessageChannelFromHostHeader(func(s string) (reference ChannelReference, err error) {
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

	err = http.WriteRequest(context.TODO(), binding.ToMessage(&event), req)
	if err != nil {
		t.Fatal(err)
	}

	res := httptest.ResponseRecorder{}

	r.ServeHTTP(&res, req)
	if res.Code != 404 {
		t.Fatal("Unexpected status code. Expected 404. Actual", res.Code)
	}
}
