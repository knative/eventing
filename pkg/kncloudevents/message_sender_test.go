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
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"

	"knative.dev/pkg/ptr"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
)

// Test The RetryConfigFromDeliverySpec() Functionality
func TestRetryConfigFromDeliverySpec(t *testing.T) {
	const retry = 5
	testcases := []struct {
		name                     string
		backoffPolicy            duckv1.BackoffPolicyType
		backoffDelay             string
		expectedBackoffDurations []time.Duration
		wantErr                  bool
	}{{
		name:          "Successful Linear Backoff 2500ms, 5 retries",
		backoffPolicy: duckv1.BackoffPolicyLinear,
		backoffDelay:  "PT2.5S",
		expectedBackoffDurations: []time.Duration{
			1 * 2500 * time.Millisecond,
			2 * 2500 * time.Millisecond,
			3 * 2500 * time.Millisecond,
			4 * 2500 * time.Millisecond,
			5 * 2500 * time.Millisecond,
		},
	}, {
		name:          "Successful Exponential Backoff 1500ms, 5 retries",
		backoffPolicy: duckv1.BackoffPolicyExponential,
		backoffDelay:  "PT1.5S",
		expectedBackoffDurations: []time.Duration{
			3 * time.Second,
			6 * time.Second,
			12 * time.Second,
			24 * time.Second,
			48 * time.Second,
		},
	}, {
		name:          "Successful Exponential Backoff 500ms, 5 retries",
		backoffPolicy: duckv1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		expectedBackoffDurations: []time.Duration{
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
			16 * time.Second,
		},
	}, {
		name:          "Invalid Backoff Delay",
		backoffPolicy: duckv1.BackoffPolicyLinear,
		backoffDelay:  "FOO",
		wantErr:       true,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Create The DeliverySpec To Test
			deliverySpec := duckv1.DeliverySpec{
				DeadLetterSink: nil,
				Retry:          ptr.Int32(retry),
				BackoffPolicy:  &tc.backoffPolicy,
				BackoffDelay:   &tc.backoffDelay,
			}

			// Create the RetryConfig from the deliverySpec
			retryConfig, err := RetryConfigFromDeliverySpec(deliverySpec)
			assert.Equal(t, tc.wantErr, err != nil)

			// If successful then validate the retryConfig (Max & Backoff calculations).
			if err == nil {
				assert.Equal(t, retry, retryConfig.RetryMax)
				for i := 1; i < retry; i++ {
					expectedBackoffDuration := tc.expectedBackoffDurations[i-1]
					actualBackoffDuration := retryConfig.Backoff(i, nil)
					assert.Equal(t, expectedBackoffDuration, actualBackoffDuration)
				}
			}
		})
	}
}

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

func TestRetriesOnNetworkErrors(t *testing.T) {

	n := int32(10)
	linear := duckv1.BackoffPolicyLinear
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

	r, err := RetryConfigFromDeliverySpec(duckv1.DeliverySpec{
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

func TestRetryIfGreaterThan300(t *testing.T) {

	// Define The TestCase Type
	type TestCase struct {
		name     string
		response *http.Response
		err      error
		result   bool
	}

	// Define The TestCases
	testCases := []TestCase{
		{
			name:   "Nil Response",
			result: true,
		},
		{
			name:     "Http Error",
			response: &http.Response{StatusCode: http.StatusOK},
			err:      errors.New("test error"),
			result:   false,
		},
		{
			name:     "Http StatusCode -1",
			response: &http.Response{StatusCode: -1},
			result:   true,
		},
		{
			name:     "Http StatusCode 100",
			response: &http.Response{StatusCode: http.StatusContinue},
			result:   false,
		},
		{
			name:     "Http StatusCode 102",
			response: &http.Response{StatusCode: http.StatusProcessing},
			result:   false,
		},
		{
			name:     "Http StatusCode 200",
			response: &http.Response{StatusCode: http.StatusOK},
			result:   false,
		},
		{
			name:     "Http StatusCode 201",
			response: &http.Response{StatusCode: http.StatusCreated},
			result:   false,
		},
		{
			name:     "Http StatusCode 202",
			response: &http.Response{StatusCode: http.StatusAccepted},
			result:   false,
		},
		{
			name:     "Http StatusCode 300",
			response: &http.Response{StatusCode: http.StatusMultipleChoices},
			result:   true,
		},
		{
			name:     "Http StatusCode 301",
			response: &http.Response{StatusCode: http.StatusMovedPermanently},
			result:   true,
		},
		{
			name:     "Http StatusCode 400",
			response: &http.Response{StatusCode: http.StatusBadRequest},
			result:   true,
		},
		{
			name:     "Http StatusCode 401",
			response: &http.Response{StatusCode: http.StatusUnauthorized},
			result:   true,
		},
		{
			name:     "Http StatusCode 403",
			response: &http.Response{StatusCode: http.StatusForbidden},
			result:   true,
		},
		{
			name:     "Http StatusCode 404",
			response: &http.Response{StatusCode: http.StatusNotFound},
			result:   true,
		},
		{
			name:     "Http StatusCode 429",
			response: &http.Response{StatusCode: http.StatusTooManyRequests},
			result:   true,
		},
		{
			name:     "Http StatusCode 500",
			response: &http.Response{StatusCode: http.StatusInternalServerError},
			result:   true,
		},
		{
			name:     "Http StatusCode 501",
			response: &http.Response{StatusCode: http.StatusNotImplemented},
			result:   true,
		},
	}

	ctx := context.TODO()

	// Execute The Individual Test Cases
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, _ := RetryIfGreaterThan300(ctx, testCase.response, testCase.err)
			assert.Equal(t, testCase.result, result)
		})
	}
}

func TestSelectiveRetry(t *testing.T) {

	// Define The TestCase Type
	type TestCase struct {
		name     string
		response *http.Response
		err      error
		result   bool
	}

	// Define The TestCases
	testCases := []TestCase{
		{
			name:   "Nil Response",
			result: true,
		},
		{
			name:     "Http Error",
			response: &http.Response{StatusCode: http.StatusOK},
			err:      errors.New("test error"),
			result:   true,
		},
		{
			name:     "Http StatusCode -1",
			response: &http.Response{StatusCode: -1},
			result:   true,
		},
		{
			name:     "Http StatusCode 100",
			response: &http.Response{StatusCode: http.StatusContinue},
			result:   false,
		},
		{
			name:     "Http StatusCode 102",
			response: &http.Response{StatusCode: http.StatusProcessing},
			result:   false,
		},
		{
			name:     "Http StatusCode 200",
			response: &http.Response{StatusCode: http.StatusOK},
			result:   false,
		},
		{
			name:     "Http StatusCode 201",
			response: &http.Response{StatusCode: http.StatusCreated},
			result:   false,
		},
		{
			name:     "Http StatusCode 202",
			response: &http.Response{StatusCode: http.StatusAccepted},
			result:   false,
		},
		{
			name:     "Http StatusCode 300",
			response: &http.Response{StatusCode: http.StatusMultipleChoices},
			result:   false,
		},
		{
			name:     "Http StatusCode 301",
			response: &http.Response{StatusCode: http.StatusMovedPermanently},
			result:   false,
		},
		{
			name:     "Http StatusCode 400",
			response: &http.Response{StatusCode: http.StatusBadRequest},
			result:   false,
		},
		{
			name:     "Http StatusCode 401",
			response: &http.Response{StatusCode: http.StatusUnauthorized},
			result:   false,
		},
		{
			name:     "Http StatusCode 403",
			response: &http.Response{StatusCode: http.StatusForbidden},
			result:   false,
		},
		{
			name:     "Http StatusCode 404",
			response: &http.Response{StatusCode: http.StatusNotFound},
			result:   true,
		},
		{
			name:     "Http StatusCode 429",
			response: &http.Response{StatusCode: http.StatusTooManyRequests},
			result:   true,
		},
		{
			name:     "Http StatusCode 500",
			response: &http.Response{StatusCode: http.StatusInternalServerError},
			result:   true,
		},
		{
			name:     "Http StatusCode 501",
			response: &http.Response{StatusCode: http.StatusNotImplemented},
			result:   true,
		},
	}

	ctx := context.TODO()

	// Execute The Individual Test Cases
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := SelectiveRetry(ctx, testCase.response, testCase.err)
			assert.Equal(t, testCase.result, result)
			assert.Nil(t, err)
		})
	}
}

func TestRetryConfigFromDeliverySpecCheckRetry(t *testing.T) {
	const retryMax = 10
	linear := eventingduck.BackoffPolicyLinear
	tests := []struct {
		name    string
		spec    eventingduck.DeliverySpec
		wantErr bool
	}{{
		name: "full delivery",
		spec: eventingduck.DeliverySpec{
			Retry:         pointer.Int32Ptr(10),
			BackoffPolicy: &linear,
			BackoffDelay:  pointer.StringPtr("PT1S"),
		},
	}, {
		name: "only retry",
		spec: eventingduck.DeliverySpec{
			Retry:         pointer.Int32Ptr(10),
			BackoffPolicy: &linear,
		},
	}, {
		name: "not ISO8601",
		spec: eventingduck.DeliverySpec{
			Retry:         pointer.Int32Ptr(10),
			BackoffDelay:  pointer.StringPtr("PP1"),
			BackoffPolicy: &linear,
		},
		wantErr: true,
	},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RetryConfigFromDeliverySpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryConfigFromDeliverySpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			if got.CheckRetry == nil {
				t.Errorf("CheckRetry must not be nil")
				return
			}
			if got.Backoff == nil {
				t.Errorf("Backoff must not be nil")
			}
			if got.RetryMax != retryMax {
				t.Errorf("RetryMax = %d, want: %d", got.RetryMax, retryMax)
			}
		})
	}
}
