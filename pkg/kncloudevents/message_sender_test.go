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
