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

// RetryAfterFormat Enum
type RetryAfterFormat int

const (
	None RetryAfterFormat = iota // RetryAfter Format Enum Values
	Seconds
	Date
	Invalid

	retryBackoffDuration = 500 * time.Millisecond // Constant (vs linear/exponential) backoff Duration for simpler validation.
	paddingDuration      = 250 * time.Millisecond // Buffer time to allow test execution to actually send the requests.
)

// RetryAfterValidationServer wraps a standard HTTP test server with tracking/validation logic.
type RetryAfterValidationServer struct {
	*httptest.Server                 // Wrapped Golang HTTP Test Server.
	previousReqTime    time.Time     // Tracking request times to validate retry intervals.
	requestCount       int32         // Tracking total requests for external validation of retry attempts.
	minRequestDuration time.Duration // Expected minimum request interval duration.
	maxRequestDuration time.Duration // Expected maximum request interval duration.
}

// newRetryAfterValidationServer returns a new RetryAfterValidationServer with the
// specified configuration. The server tracks total request counts and validates
// inter-request durations to ensure they confirm to the expected backoff behavior.
func newRetryAfterValidationServer(t *testing.T, statusCode int, retryAfterFormat RetryAfterFormat, retryAfterDuration time.Duration, requestDuration time.Duration) *RetryAfterValidationServer {

	server := &RetryAfterValidationServer{
		minRequestDuration: requestDuration,
		maxRequestDuration: requestDuration + paddingDuration,
	}

	server.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {

		// Determine Time Of Current Request
		currentReqTime := time.Now()

		// Adjust Expected Min Request Duration To Account For RFC850 Date Format Milliseconds Truncation
		// TODO - Remove this check when experimental-feature moves to Stable/GA to convert behavior from opt-in to opt-out
		if server.minRequestDuration > retryBackoffDuration {
			// TODO - Keep this logic as is (no change required) when experimental-feature moves to Stable/GA
			if retryAfterFormat == Date {
				truncatedDuration := currentReqTime.Sub(currentReqTime.Truncate(time.Second))
				server.minRequestDuration = server.minRequestDuration - truncatedDuration
			}
		}

		// Count The Requests
		atomic.AddInt32(&server.requestCount, 1)

		// Add Retry-After Header To Response Based On Configured Format
		switch retryAfterFormat {
		case None:
			// Don't Write Anything ;)
		case Seconds:
			writer.Header().Set(RetryAfterHeader, strconv.Itoa(int(retryAfterDuration.Seconds())))
		case Date:
			writer.Header().Set(RetryAfterHeader, currentReqTime.Add(retryAfterDuration).Format(time.RFC850)) // RFC850 Drops Millis
		case Invalid:
			writer.Header().Set(RetryAfterHeader, "FOO")
		default:
			assert.Fail(t, "TestCase with unsupported ResponseFormat '%v'", retryAfterFormat)
		}

		// Only Validate Timings Of Retries (Skip Initial Request)
		if server.requestCount > 1 {

			// Validate Inter-Request Durations Meet Or Exceed Expected Minimums
			actualRequestDuration := currentReqTime.Sub(server.previousReqTime)
			assert.GreaterOrEqual(t, actualRequestDuration, server.minRequestDuration, "previousReqTime =", server.previousReqTime.String(), "currentReqTime =", currentReqTime.String())
			assert.LessOrEqual(t, actualRequestDuration, server.maxRequestDuration, "previousReqTime =", server.previousReqTime.String(), "currentReqTime =", currentReqTime.String())
			t.Logf("Request Duration %s should be within expected range %s - %s",
				actualRequestDuration.String(),
				server.minRequestDuration.String(),
				server.maxRequestDuration.String())
		}

		// Respond With StatusCode & Cycle The Times
		writer.WriteHeader(statusCode)
		server.previousReqTime = currentReqTime
	}))

	return server
}

/*
 *  Test Retry-After Header Enforcement
 *
 * This test validates that SendWithRetries() is correctly enforcing
 * the Retry-After headers based on RetryConfig.RetryAfterMaxDuration.
 * It does this be creating a test HTTP Server which responds with
 * the desired StatusCode and Retry-After header.  The server also
 * validates the retry request intervals to ensure they fall within
 * the expected time window.  The timings were chosen as a balance of
 * stability and test execution speed, but could require adjustment.
 */
func TestHTTPMessageSenderSendWithRetriesWithRetryAfter(t *testing.T) {

	// Test Data
	retryMax := int32(2)                           // Perform a couple of retries to be sure.
	smallRetryAfterMaxDuration := 1 * time.Second  // Value must exceed retryBackoffDuration while being less than retryAfterDuration so that the retryAfterMax value is used.
	largeRetryAfterMaxDuration := 10 * time.Second // Value must exceed retryBackoffDuration and retryAfterDuration so that Retry-After header is used.

	// Define The TestCases
	testCases := []struct {
		name                  string
		statusCode            int              // HTTP StatusCode which the server should return.
		retryAfterFormat      RetryAfterFormat // Format in which the server should return Retry-After headers.
		retryAfterDuration    time.Duration    // Duration of the Retry-After header returned by the server.
		retryAfterMaxDuration *time.Duration   // DeliverySpec RetryAfterMax Duration used to calculate expected retry interval.
		wantRequestDuration   time.Duration    // Expected minimum Request interval Duration.
	}{

		// Nil Max Tests (opt-in / opt-out)

		{
			name:                  "default max 429 without Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "default max 429 with Retry-After seconds",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    1 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration, // TODO - Update when experimental-feature moves to Stable/GA
		},
		{
			name:                  "default max 429 with Retry-After date",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Date,
			retryAfterDuration:    2 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration, // TODO - Update when experimental-feature moves to Stable/GA
		},
		{
			name:                  "default max 429 with invalid Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Invalid,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "default max 503 with Retry-After seconds",
			statusCode:            http.StatusServiceUnavailable,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    1 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration, // TODO - Update when experimental-feature moves to Stable/GA
		},
		{
			name:                  "default max 500 without Retry-After",
			statusCode:            http.StatusInternalServerError,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: nil,
			wantRequestDuration:   retryBackoffDuration,
		},

		// Large Max Tests (Greater Than Retry-After Value)

		{
			name:                  "large max 429 without Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "large max 429 with Retry-After seconds",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    1 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   1 * time.Second,
		},
		{
			name:                  "large max 429 with Retry-After date",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Date,
			retryAfterDuration:    2 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   2 * time.Second,
		},
		{
			name:                  "large max 429 with invalid Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Invalid,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "large max 503 with Retry-After seconds",
			statusCode:            http.StatusServiceUnavailable,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    1 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   1 * time.Second,
		},
		{
			name:                  "large max 500 without Retry-After",
			statusCode:            http.StatusInternalServerError,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &largeRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
		},

		// Small Max Tests (Less Than Retry-After Value)

		{
			name:                  "small max 429 without Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "small max 429 with Retry-After seconds",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    4 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   smallRetryAfterMaxDuration,
		},
		{
			name:                  "small max 429 with Retry-After date",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Date,
			retryAfterDuration:    2 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   smallRetryAfterMaxDuration,
		},
		{
			name:                  "small max 429 with invalid Retry-After",
			statusCode:            http.StatusTooManyRequests,
			retryAfterFormat:      Invalid,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
		},
		{
			name:                  "small max 503 with Retry-After seconds",
			statusCode:            http.StatusServiceUnavailable,
			retryAfterFormat:      Seconds,
			retryAfterDuration:    4 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   smallRetryAfterMaxDuration,
		},
		{
			name:                  "small max 500 without Retry-After",
			statusCode:            http.StatusInternalServerError,
			retryAfterFormat:      None,
			retryAfterDuration:    0 * time.Second,
			retryAfterMaxDuration: &smallRetryAfterMaxDuration,
			wantRequestDuration:   retryBackoffDuration,
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

			// Create A RetryAfter Validation Server To Validate Retry Durations
			server := newRetryAfterValidationServer(t, tc.statusCode, tc.retryAfterFormat, tc.retryAfterDuration, tc.wantRequestDuration)

			// Create RetryConfig With RetryAfterMax From TestCase
			retryConfig := RetryConfig{
				RetryMax:              int(retryMax),
				CheckRetry:            SelectiveRetry,
				Backoff:               func(attemptNum int, resp *http.Response) time.Duration { return retryBackoffDuration },
				RetryAfterMaxDuration: tc.retryAfterMaxDuration,
			}

			// Perform The Test - Generate And Send The Initial Request
			t.Logf("Testing %d Response With RetryAfter (%d) %fs & Max %+v", tc.statusCode, tc.retryAfterFormat, tc.retryAfterDuration.Seconds(), tc.retryAfterMaxDuration)
			sender := &HTTPMessageSender{Client: http.DefaultClient}
			request, err := http.NewRequest("POST", server.URL, nil)
			assert.Nil(t, err)
			response, err := sender.SendWithRetries(request, &retryConfig)

			// Verify Final Results (Actual Retry Timing Validated In Server)
			assert.Nil(t, err)
			assert.Equal(t, response.StatusCode, tc.statusCode)
			assert.Equal(t, retryMax+1, atomic.LoadInt32(&server.requestCount))
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
