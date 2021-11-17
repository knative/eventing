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
	"strconv"
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

func TestHTTPMessageSenderSendWithRetriesWithSingleRequestTimeout(t *testing.T) {
	t.Parallel()

	timeout := time.Second * 3

	var n int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		newVal := atomic.AddInt32(&n, 1)
		if newVal >= 5 {
			writer.WriteHeader(http.StatusOK)
		} else {
			// Let's add a bit more time
			time.Sleep(timeout + (200 * time.Millisecond))
			writer.WriteHeader(http.StatusAccepted)
		}
	}))
	defer server.Close()

	sender := &HTTPMessageSender{
		Client: getClient(),
	}
	config := &RetryConfig{
		RetryMax:   5,
		CheckRetry: RetryIfGreaterThan300,
		Backoff: func(attemptNum int, resp *http.Response) time.Duration {
			return time.Millisecond
		},
		RequestTimeout: timeout,
	}

	request, err := http.NewRequest("POST", server.URL, nil)
	require.NoError(t, err)

	got, err := sender.SendWithRetries(request, config)

	require.Equal(t, 5, int(atomic.LoadInt32(&n)))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, got.StatusCode)
}

/*
 *  Test Retry-After Header Enforcement
 *
 *  Note - This test is a bit complicated in that it utilizes a custom
 *         test Server which verifies subsequent retry attempts are
 *         within an expected time window.  It is therefore inherently
 *         "time" sensitive and could become brittle.  The timings
 *         were chosen as a balance of stability and test execution
 *         speed, but could require adjustment.
 */
func TestHTTPMessageSenderSendWithRetriesWithRetryAfter(t *testing.T) {

	// Create A Quick Enum For RetryAfter Format
	type RetryAfterFormat int
	const (
		None RetryAfterFormat = iota
		Seconds
		Date
		Invalid
	)

	// Test Data
	smallRetryAfterMaxDuration := 1 * time.Second
	largeRetryAfterMaxDuration := 10 * time.Second

	// Define The TestCases
	tests := []struct {
		name            string
		config          *RetryConfig     // The RetryConfig to use in for the test case.
		statusCode      int              // The HTTP StatusCode which the Server should return.
		responseFormat  RetryAfterFormat // Indicates the format in which the Server should return the Retry-After header.
		responseSeconds int              // Indicates the number of Seconds for the Server to use when returning the Retry-After header.
		wantReqCount    int              // The total number of expected requests, including initial request.
	}{
		//
		// Default Max Tests (0 Duration opt-in vs opt-out)
		//

		{
			name: "default max 429 without Retry-After",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: None,
			wantReqCount:   3,
		},
		{
			name: "default max 429 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Seconds,
			responseSeconds: 1,
			wantReqCount:    3,
		},
		{
			name: "default max 429 with Retry-After date",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Date,
			responseSeconds: 2,
			wantReqCount:    3,
		},
		{
			name: "default max 429 with invalid Retry-After",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: Invalid,
			wantReqCount:   3,
		},
		{
			name: "default max 503 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:      http.StatusServiceUnavailable,
			responseFormat:  Seconds,
			responseSeconds: 1,
			wantReqCount:    3,
		},
		{
			name: "default max 500 without Retry-After",
			config: &RetryConfig{
				RetryMax: 2,
			},
			statusCode:     http.StatusInternalServerError,
			responseFormat: None,
			wantReqCount:   3,
		},

		//
		// Large Max Tests (Greater Than Retry-After Value)
		//

		{
			name: "large max 429 without Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: None,
			wantReqCount:   3,
		},
		{
			name: "large max 429 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Seconds,
			responseSeconds: 1,
			wantReqCount:    3,
		},
		{
			name: "large max 429 with Retry-After date",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Date,
			responseSeconds: 1,
			wantReqCount:    3,
		},
		{
			name: "large max 429 with invalid Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: Invalid,
			wantReqCount:   3,
		},
		{
			name: "large max 503 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:      http.StatusServiceUnavailable,
			responseFormat:  Seconds,
			responseSeconds: 1,
			wantReqCount:    3,
		},
		{
			name: "large max 500 without Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &largeRetryAfterMaxDuration,
			},
			statusCode:     http.StatusInternalServerError,
			responseFormat: None,
			wantReqCount:   3,
		},

		//
		// Small Max Tests (Less Than Retry-After Value)
		//

		{
			name: "small max 429 without Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: None,
			wantReqCount:   3,
		},
		{
			name: "small max 429 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Seconds,
			responseSeconds: 4,
			wantReqCount:    3,
		},
		{
			name: "small max 429 with Retry-After date",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:      http.StatusTooManyRequests,
			responseFormat:  Date,
			responseSeconds: 2,
			wantReqCount:    3,
		},
		{
			name: "small max 429 with invalid Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:     http.StatusTooManyRequests,
			responseFormat: Invalid,
			wantReqCount:   3,
		},
		{
			name: "small max 503 with Retry-After seconds",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:      http.StatusServiceUnavailable,
			responseFormat:  Seconds,
			responseSeconds: 4,
			wantReqCount:    3,
		},
		{
			name: "small max 500 without Retry-After",
			config: &RetryConfig{
				RetryMax:              2,
				RetryAfterMaxDuration: &smallRetryAfterMaxDuration,
			},
			statusCode:     http.StatusInternalServerError,
			responseFormat: None,
			wantReqCount:   3,
		},
	}

	// Execute The TestCases
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			t.Parallel()

			// Consistent CheckRetry & Backoff Implementation For All TestCases
			paddingDuration := 250 * time.Millisecond // Test Execution Allowance For Resending Requests
			backoffDuration := 500 * time.Millisecond // Constant Backoff Time For Simplified Testing
			tc.config.CheckRetry = SelectiveRetry
			tc.config.Backoff = func(attemptNum int, resp *http.Response) time.Duration { return backoffDuration }

			// Determine The Response Retry-After Backoff Duration From TestCase
			var responseDuration time.Duration
			switch tc.responseFormat {
			case None, Invalid:
			case Seconds, Date:
				if tc.responseSeconds > 0 {
					responseDuration = time.Duration(tc.responseSeconds) * time.Second
				}
			default:
				assert.Fail(t, "TestCase with unsupported ResponseFormat '%v'", tc.responseFormat)
			}

			// Tracking Variables
			var previousReqTime time.Time
			var reqCount int32

			// Create A Test HTTP Server Capable Of Validating Request Interim Durations
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

				// Determine Time Of Current Request
				currentReqTime := time.Now()
				// TODO - Remove this check when experimental-feature moves to Stable/GA to convert behavior from opt-in to opt-out
				if tc.config.RetryAfterMaxDuration != nil {

					// TODO - Keep this logic as is (no change required) when experimental-feature is Stable/GA
					if tc.responseFormat == Date && tc.responseSeconds > 0 {
						currentReqTime = currentReqTime.Round(time.Second) // Round When Using Date Format To Account For RFC850 Precision
					}
				}

				// Count The Requests
				atomic.AddInt32(&reqCount, 1)

				// Add Retry-After Header To Response Based On TestCase
				switch tc.responseFormat {
				case None:
					// Don't Write Anything ;)
				case Seconds:
					writer.Header().Set(RetryAfterHeader, strconv.Itoa(int(responseDuration.Seconds())))
				case Date:
					writer.Header().Set(RetryAfterHeader, currentReqTime.Add(responseDuration).Format(time.RFC850)) // RFC850 Drops Millis
				case Invalid:
					writer.Header().Set(RetryAfterHeader, "FOO")
				default:
					assert.Fail(t, "TestCase with unsupported ResponseFormat '%v'", tc.responseFormat)
				}

				// Only Validate Timings Of Retries - Not Interested In Initial Request
				if reqCount > 1 {

					// Calculate The Expected Maximum Request Duration Of TestCase
					expectedMinRequestDuration := backoffDuration
					// TODO - Remove this check when experimental-feature moves to Stable/GA to convert behavior from opt-in to opt-out
					if tc.config.RetryAfterMaxDuration != nil {

						// TODO - Keep this logic as is (no change required) when experimental-feature is Stable/GA
						if responseDuration > 0 {
							expectedMinRequestDuration = responseDuration
							if tc.config.RetryAfterMaxDuration != nil {
								if *tc.config.RetryAfterMaxDuration == 0 {
									expectedMinRequestDuration = backoffDuration
								} else if *tc.config.RetryAfterMaxDuration < expectedMinRequestDuration {
									expectedMinRequestDuration = *tc.config.RetryAfterMaxDuration
								}
							}
						}
					}
					expectedMaxRequestDuration := expectedMinRequestDuration + paddingDuration

					// Validate Inter-Request Durations Meet Or Exceed Expected Minimums
					actualRequestDuration := currentReqTime.Sub(previousReqTime)
					assert.GreaterOrEqual(t, actualRequestDuration, expectedMinRequestDuration, "previousReqTime =", previousReqTime.String(), "currentReqTime =", currentReqTime.String())
					assert.LessOrEqual(t, actualRequestDuration, expectedMaxRequestDuration, "previousReqTime =", previousReqTime.String(), "currentReqTime =", currentReqTime.String())
					t.Logf("Validated Request Duration %s between expected range %s - %s",
						actualRequestDuration.String(),
						expectedMinRequestDuration.String(),
						expectedMaxRequestDuration.String())
				}

				// Respond With StatusCode From TestCase & Cycle The Times
				writer.WriteHeader(tc.statusCode)
				previousReqTime = currentReqTime
			}))

			// Perform The Test - Generate And Send The Request
			t.Logf("Testing %d Response With RetryAfter (%d) %ds & Max %+v", tc.statusCode, tc.responseFormat, tc.responseSeconds, tc.config.RetryAfterMaxDuration)
			sender := &HTTPMessageSender{Client: http.DefaultClient}
			request, err := http.NewRequest("POST", server.URL, nil)
			assert.Nil(t, err)
			got, err := sender.SendWithRetries(request, tc.config)

			// Verify Final Results
			if err != nil {
				t.Fatalf("SendWithRetries() error = %v, wantErr nil", err)
			}
			if got.StatusCode != tc.statusCode {
				t.Fatalf("SendWithRetries() got = %v, want %v", got.StatusCode, tc.statusCode)
			}
			if int(atomic.LoadInt32(&reqCount)) != tc.wantReqCount {
				t.Fatalf("expected %d retries got %d", tc.config.RetryMax, reqCount)
			}
		})
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
