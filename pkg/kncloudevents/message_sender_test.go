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
	nethttp "net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
)

// Test The RetryConfigFromDeliverySpec() Functionality
func TestRetryConfigFromDeliverySpec(t *testing.T) {

	// Define The TestCase Structure
	type TestCase struct {
		name                     string
		retry                    int32
		backoffPolicy            duckv1.BackoffPolicyType
		backoffDelay             string
		expectedBackoffDurations []time.Duration
		wantErr                  bool
	}

	// Create The TestCases
	testcases := []TestCase{
		{
			name:          "Successful Linear Backoff 2500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyLinear,
			backoffDelay:  "PT2.5S",
			expectedBackoffDurations: []time.Duration{
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name:          "Successful Exponential Backoff 1500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyExponential,
			backoffDelay:  "PT1.5S",
			expectedBackoffDurations: []time.Duration{
				3 * time.Second,
				6 * time.Second,
				12 * time.Second,
				24 * time.Second,
				48 * time.Second,
			},
			wantErr: false,
		},
		{
			name:          "Successful Exponential Backoff 500ms",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyExponential,
			backoffDelay:  "PT0.5S",
			expectedBackoffDurations: []time.Duration{
				1 * time.Second,
				2 * time.Second,
				4 * time.Second,
				8 * time.Second,
				16 * time.Second,
			},
			wantErr: false,
		},
		{
			name:          "Invalid Backoff Delay",
			retry:         int32(5),
			backoffPolicy: duckv1.BackoffPolicyLinear,
			backoffDelay:  "FOO",
			wantErr:       true,
		},
	}

	// Loop Over The TestCases
	for _, testcase := range testcases {

		// Execute The TestCase
		t.Run(testcase.name, func(t *testing.T) {

			// Create The DeliverySpec To Test
			deliverySpec := duckv1.DeliverySpec{
				DeadLetterSink: nil,
				Retry:          &testcase.retry,
				BackoffPolicy:  &testcase.backoffPolicy,
				BackoffDelay:   &testcase.backoffDelay,
			}

			// Create The RetryConfig From The DeliverySpec
			retryConfig, err := RetryConfigFromDeliverySpec(deliverySpec)
			assert.Equal(t, testcase.wantErr, err != nil)

			// If Successful Then Validate The RetryConfig (Max & Backoff Calculations)
			if err == nil {
				assert.Equal(t, int(testcase.retry), retryConfig.RetryMax)
				for i := 1; i < int(testcase.retry); i++ {
					expectedBackoffDuration := testcase.expectedBackoffDurations[i-1]
					actualBackoffDuration := retryConfig.Backoff(i, nil)
					assert.Equal(t, expectedBackoffDuration, actualBackoffDuration)
				}
			}
		})
	}
}

func TestHttpMessageSenderSendWithRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       *RetryConfig
		wantStatus   int
		wantDispatch int
		wantErr      bool
	}{
		{
			name: "5 max retry",
			config: &RetryConfig{
				RetryMax: 5,
				CheckRetry: func(ctx context.Context, resp *nethttp.Response, err error) (bool, error) {
					return true, nil
				},
				Backoff: func(attemptNum int, resp *nethttp.Response) time.Duration {
					return time.Millisecond
				},
			},
			wantStatus:   nethttp.StatusServiceUnavailable,
			wantDispatch: 6,
			wantErr:      false,
		},
		{
			name: "1 max retry",
			config: &RetryConfig{
				RetryMax: 1,
				CheckRetry: func(ctx context.Context, resp *nethttp.Response, err error) (bool, error) {
					return true, nil
				},
				Backoff: func(attemptNum int, resp *nethttp.Response) time.Duration {
					return time.Millisecond
				},
			},
			wantStatus:   nethttp.StatusServiceUnavailable,
			wantDispatch: 2,
			wantErr:      false,
		},
		{
			name:         "with no retryConfig",
			wantStatus:   nethttp.StatusServiceUnavailable,
			wantDispatch: 1,
			wantErr:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			n := 0
			var mu sync.Mutex
			server := httptest.NewServer(nethttp.HandlerFunc(func(writer nethttp.ResponseWriter, request *nethttp.Request) {
				mu.Lock()
				n++
				mu.Unlock()

				writer.WriteHeader(tt.wantStatus)
			}))

			sender := &HttpMessageSender{
				Client: nethttp.DefaultClient,
			}

			request, err := nethttp.NewRequest("POST", server.URL, nil)
			assert.Nil(t, err)
			got, err := sender.SendWithRetries(request, tt.config)
			if (err != nil) != tt.wantErr || got == nil {
				t.Errorf("SendWithRetries() error = %v, wantErr %v or got nil", err, tt.wantErr)
				return
			}
			if got.StatusCode != nethttp.StatusServiceUnavailable {
				t.Errorf("SendWithRetries() got = %v, want %v", got.StatusCode, nethttp.StatusServiceUnavailable)
				return
			}
			if n != tt.wantDispatch {
				t.Errorf("expected %d retries got %d", tt.config.RetryMax, n)
				return
			}
		})
	}
}

func TestRetryConfigFromDeliverySpecCheckRetry(t *testing.T) {
	linear := eventingduck.BackoffPolicyLinear
	tests := []struct {
		name     string
		spec     eventingduck.DeliverySpec
		retryMax int
		wantErr  bool
	}{
		{
			name: "full delivery",
			spec: eventingduck.DeliverySpec{
				Retry:         pointer.Int32Ptr(10),
				BackoffPolicy: &linear,
				BackoffDelay:  pointer.StringPtr("PT1S"),
			},
			retryMax: 10,
			wantErr:  false,
		},
		{
			name: "only retry",
			spec: eventingduck.DeliverySpec{
				Retry:         pointer.Int32Ptr(10),
				BackoffPolicy: &linear,
			},
			retryMax: 10,
			wantErr:  false,
		},
		{
			name: "not ISO8601",
			spec: eventingduck.DeliverySpec{
				Retry:         pointer.Int32Ptr(10),
				BackoffDelay:  pointer.StringPtr("PP1"),
				BackoffPolicy: &linear,
			},
			retryMax: 10,
			wantErr:  true,
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
			if got.RetryMax != tt.retryMax {
				t.Errorf("retryMax want %d got %d", tt.retryMax, got.RetryMax)
			}
		})
	}
}
