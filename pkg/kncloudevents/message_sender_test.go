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
		CheckRetry: SelectiveRetry,
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

// RetryAfterFormat Enum
type RetryAfterFormat int

const (
	None RetryAfterFormat = iota // RetryAfter Format Enum Values
	Seconds
	Date
	Invalid
)

/*
 * Test TestGenerateBackoffFnWithRetryAfter
 *
 * This test validates that generateBackoffFn() is correctly enforcing
 * the Retry-After headers based on RetryConfig.RetryAfterMaxDuration.
 * This is tested directly, rather than indirectly via SendWithRetries(),
 * due to the complexity in ensuring timing checks of sequential requests.
 * The CI pipeline is not reliable enough to consistently ensure any
 * specific response times, making the tests inherently flaky during slow
 * test runs. This version is also much faster to execute as it doesn't
 * to stand-up test servers.
 */
func TestGenerateBackoffFnWithRetryAfter(t *testing.T) {

	// Test Data
	retryBackoffDuration := 2 * time.Second        // The standard constant (non Retry-After) backoff Duration.
	retryAfterDuration := 30 * time.Second         // The Retry-After header Duration to use in HTTP Response.
	smallRetryAfterMaxDuration := 10 * time.Second // Value must exceed retryBackoffDuration while being less than retryAfterDuration to force use of retryAfterMax value.
	largeRetryAfterMaxDuration := 90 * time.Second // Value must exceed retryBackoffDuration and retryAfterDuration so that Retry-After header is used.

	// Define The TestCases
	testCases := []struct {
		name            string
		retryAfterMax   *time.Duration
		statusCode      int
		format          RetryAfterFormat
		expectedBackoff time.Duration
	}{
		// Nil Max Tests (opt-in / opt-out)

		{
			name:            "nil max 429 without Retry-After",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "nil max 429 with Retry-After seconds",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          Seconds,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "nil max 429 with Retry-After date",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          Date,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "nil max 429 with invalid Retry-After",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          Invalid,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "nil max 503 with Retry-After seconds",
			retryAfterMax:   nil,
			statusCode:      http.StatusServiceUnavailable,
			format:          Seconds,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "nil max 500 without Retry-After",
			retryAfterMax:   nil,
			statusCode:      http.StatusInternalServerError,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},

		// Large Max Tests (Greater Than Retry-After Value)

		{
			name:            "large max 429 without Retry-After",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "large max 429 with Retry-After seconds",
			retryAfterMax:   &largeRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Seconds,
			expectedBackoff: retryAfterDuration, // Respects Retry-After Header
		},
		{
			name:            "large max 429 with Retry-After date",
			retryAfterMax:   &largeRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Date,
			expectedBackoff: retryAfterDuration, // Respects Retry-After Header
		},
		{
			name:            "large max 429 with invalid Retry-After",
			retryAfterMax:   &largeRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Invalid,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "large max 503 with Retry-After seconds",
			retryAfterMax:   &largeRetryAfterMaxDuration,
			statusCode:      http.StatusServiceUnavailable,
			format:          Seconds,
			expectedBackoff: retryAfterDuration, // Respects Retry-After Header
		},
		{
			name:            "large max 500 without Retry-After",
			retryAfterMax:   &largeRetryAfterMaxDuration,
			statusCode:      http.StatusInternalServerError,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},

		// Small Max Tests (Less Than Retry-After Value)

		{
			name:            "small max 429 without Retry-After",
			retryAfterMax:   nil,
			statusCode:      http.StatusTooManyRequests,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "small max 429 with Retry-After seconds",
			retryAfterMax:   &smallRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Seconds,
			expectedBackoff: smallRetryAfterMaxDuration, // Respects RetryAfterMax Config
		},
		{
			name:            "small max 429 with Retry-After date",
			retryAfterMax:   &smallRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Date,
			expectedBackoff: smallRetryAfterMaxDuration, // Respects RetryAfterMax Config
		},
		{
			name:            "small max 429 with invalid Retry-After",
			retryAfterMax:   &smallRetryAfterMaxDuration,
			statusCode:      http.StatusTooManyRequests,
			format:          Invalid,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
		{
			name:            "small max 503 with Retry-After seconds",
			retryAfterMax:   &smallRetryAfterMaxDuration,
			statusCode:      http.StatusServiceUnavailable,
			format:          Seconds,
			expectedBackoff: smallRetryAfterMaxDuration, // Respects RetryAfterMax Config
		},
		{
			name:            "small max 500 without Retry-After",
			retryAfterMax:   &smallRetryAfterMaxDuration,
			statusCode:      http.StatusInternalServerError,
			format:          None,
			expectedBackoff: retryBackoffDuration, // Uses Standard Backoff
		},
	}

	// Loop Over The TestCases
	for _, testCase := range testCases {

		// Capture Range Variable For Parallel Execution
		tc := testCase

		// Execute The Individual TestCase
		t.Run(tc.name, func(t *testing.T) {

			// Run TestCases In Parallel
			t.Parallel()

			// Create RetryConfig With RetryAfterMax From TestCase
			retryConfig := &RetryConfig{
				RetryMax:              1,
				CheckRetry:            SelectiveRetry,
				Backoff:               func(_ int, _ *http.Response) time.Duration { return retryBackoffDuration },
				RetryAfterMaxDuration: tc.retryAfterMax,
			}

			// Generate An HTTP Response Based On TestCase Specification
			response := generateRetryAfterHttpResponse(t, tc.statusCode, tc.format, retryAfterDuration)

			// Generate The Backoff Function To Test
			backoffFn := generateBackoffFn(retryConfig)

			// Perform The Test
			startTime := time.Now()
			actualBackoff := backoffFn(0, 999, 1, response)
			stopTime := time.Now()

			// Verify Results
			if tc.format == Date {
				// The "time.Until()" operation is difficult to test on inconsistent/slow build
				// infrastructure, and so we have to gate the check of the lower-bounds in Date
				// format tests.  This should perform the comparison most of the time, but will
				// prevent false-positive failures on slow execution runs.
				if tc.retryAfterMax != nil && stopTime.Sub(startTime) < retryAfterDuration {
					assert.Greater(t, actualBackoff, retryBackoffDuration)
				}
				assert.LessOrEqual(t, actualBackoff, tc.expectedBackoff)
			} else {
				assert.Equal(t, tc.expectedBackoff, actualBackoff)
			}
		})
	}
}

// generateRetryAfterHttpResponse is a utility function for generating tst HTTP Responses with Retry-After headers.
func generateRetryAfterHttpResponse(t *testing.T, statusCode int, format RetryAfterFormat, duration time.Duration) *http.Response {

	// Create The Http Response With Specified StatusCode
	response := &http.Response{
		StatusCode: statusCode,
		Header:     map[string][]string{},
	}

	// Add Retry-After Header To Response Based On Configured Format
	switch format {
	case None:
		// Don't Write Any Headers ;)
	case Seconds:
		response.Header[RetryAfterHeader] = []string{strconv.Itoa(int(duration.Seconds()))}
	case Date:
		response.Header[RetryAfterHeader] = []string{time.Now().Add(duration).Format(time.RFC850)} // RFC850 Drops Millis
	case Invalid:
		response.Header[RetryAfterHeader] = []string{"FOO"}
	default:
		assert.Fail(t, "TestCase with unsupported ResponseFormat '%v'", format)
	}

	// Return The HTTP Response
	return response
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
		Retry:         pointer.Int32(n),
		BackoffPolicy: &linear,
		BackoffDelay:  pointer.String("PT0.1S"),
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
