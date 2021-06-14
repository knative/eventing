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

package kncloudevents

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
)

func TestHTTPMessageSenderSendWithRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       *RetryConfig
		wantStatus   int
		wantDispatch int
	}{{
		name: "5 max retry",
		config: &RetryConfig{
			RetryMax: 5,
			CheckRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) {
				return true, nil
			},
			Backoff: func(attemptNum int, resp *http.Response) time.Duration {
				return time.Millisecond
			},
		},
		wantStatus:   http.StatusServiceUnavailable,
		wantDispatch: 6,
	}, {
		name: "1 max retry",
		config: &RetryConfig{
			RetryMax: 1,
			CheckRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) {
				return true, nil
			},
			Backoff: func(attemptNum int, resp *http.Response) time.Duration {
				return time.Millisecond
			},
		},
		wantStatus:   http.StatusServiceUnavailable,
		wantDispatch: 2,
	}, {
		name:         "with no retryConfig",
		wantStatus:   http.StatusServiceUnavailable,
		wantDispatch: 1,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var n int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				atomic.AddInt32(&n, 1)
				writer.WriteHeader(tt.wantStatus)
			}))

			sender := &HTTPMessageSender{
				Client: http.DefaultClient,
			}

			request, err := http.NewRequest("POST", server.URL, nil)
			assert.Nil(t, err)
			got, err := sender.SendWithRetries(request, tt.config)
			if err != nil {
				t.Fatalf("SendWithRetries() error = %v, wantErr nil", err)
			}
			if got.StatusCode != http.StatusServiceUnavailable {
				t.Fatalf("SendWithRetries() got = %v, want %v", got.StatusCode, http.StatusServiceUnavailable)
			}
			if count := int(atomic.LoadInt32(&n)); count != tt.wantDispatch {
				t.Fatalf("expected %d retries got %d", tt.config.RetryMax, count)
			}
		})
	}
}

func TestHTTPMessageSenderSendWithRetriesWithBufferedMessage(t *testing.T) {
	t.Parallel()

	const wantToSkip = 9
	config := &RetryConfig{
		RetryMax: wantToSkip,
		CheckRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) {
			return true, nil
		},
		Backoff: func(attemptNum int, resp *http.Response) time.Duration {
			return time.Millisecond * 50 * time.Duration(attemptNum)
		},
	}

	var n uint32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		thisReqN := atomic.AddUint32(&n, 1)
		if thisReqN <= wantToSkip {
			writer.WriteHeader(http.StatusServiceUnavailable)
		} else {
			writer.WriteHeader(http.StatusAccepted)
		}
	}))

	sender := &HTTPMessageSender{
		Client: http.DefaultClient,
	}

	request, err := http.NewRequest("POST", server.URL, nil)
	assert.Nil(t, err)

	// Create a message similar to the one we send with channels
	mockMessage := bindingtest.MustCreateMockBinaryMessage(cetest.FullEvent())
	bufferedMessage, err := buffering.BufferMessage(context.TODO(), mockMessage)
	assert.Nil(t, err)

	err = cehttp.WriteRequest(context.TODO(), bufferedMessage, request)
	assert.Nil(t, err)

	got, err := sender.SendWithRetries(request, config)
	if err != nil {
		t.Fatalf("SendWithRetries() error = %v, wantErr nil", err)
	}
	if got.StatusCode != http.StatusAccepted {
		t.Fatalf("SendWithRetries() got = %v, want %v", got.StatusCode, http.StatusAccepted)
	}
	if count := atomic.LoadUint32(&n); count != wantToSkip+1 {
		t.Fatalf("expected %d count got %d", wantToSkip+1, count)
	}
}

func TestRetriesOnNetworkErrors(t *testing.T) {

	n := int32(10)
	linear := v1.BackoffPolicyLinear
	target := "127.0.0.1:63468"

	calls := make(chan struct{})
	defer close(calls)

	nCalls := int32(0)

	cont := make(chan struct{})
	defer close(cont)

	go func() {
		for range calls {

			nCalls++
			// Simulate that the target service is back up.
			//
			// First n/2-1 calls we get connection refused since there is no server running.
			// Now we start a server that responds with a retryable error, so we expect that
			// the client continues to retry for a different reason.
			//
			// The last time we return 200, so we don't expect a new retry.
			if n/2 == nCalls {

				l, err := net.Listen("tcp", target)
				assert.Nil(t, err)

				s := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if n-1 != nCalls {
						writer.WriteHeader(http.StatusServiceUnavailable)
						return
					}
				}))
				defer s.Close() //nolint // defers in this range loop won't run unless the channel gets closed

				assert.Nil(t, s.Listener.Close())

				s.Listener = l

				s.Start()
			}
			cont <- struct{}{}
		}
	}()

	r, err := RetryConfigFromDeliverySpec(v1.DeliverySpec{
		Retry:         pointer.Int32Ptr(n),
		BackoffPolicy: &linear,
		BackoffDelay:  pointer.StringPtr("PT0.1S"),
	})
	assert.Nil(t, err)

	checkRetry := r.CheckRetry

	r.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		calls <- struct{}{}
		<-cont

		return checkRetry(ctx, resp, err)
	}

	req, err := http.NewRequest("POST", "http://"+target, nil)
	assert.Nil(t, err)

	sender, err := NewHTTPMessageSenderWithTarget("")
	assert.Nil(t, err)

	_, err = sender.SendWithRetries(req, &r)
	assert.Nil(t, err)

	// nCalls keeps track of how many times a call to check retry occurs.
	// Since the number of request are n + 1 and the last one is successful the expected number of calls are n.
	assert.Equal(t, n, nCalls, "expected %d got %d", n, nCalls)
}

func TestHTTPMessageSender_NewCloudEventRequestWithTarget(t *testing.T) {
	s := &HTTPMessageSender{
		Client: getClient(),
		Target: "localhost",
	}

	expectedUrl, err := url.Parse("example.com")
	require.NoError(t, err)
	req, err := s.NewCloudEventRequestWithTarget(context.TODO(), "example.com")
	require.NoError(t, err)
	require.Equal(t, req.URL, expectedUrl)
}

func TestHTTPMessageSender_NewCloudEventRequest(t *testing.T) {
	s := &HTTPMessageSender{
		Client: getClient(),
		Target: "localhost",
	}

	expectedUrl, err := url.Parse("localhost")
	require.NoError(t, err)
	req, err := s.NewCloudEventRequest(context.TODO())
	require.NoError(t, err)
	require.Equal(t, req.URL, expectedUrl)
}
