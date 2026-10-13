/*
Copyright 2021 The Knative Authors

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
	"net/http"
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rickb777/date/period"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
	"knative.dev/pkg/ptr"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
)

// RetryAfterFormat Enum
type RetryAfterFormat int

const (
	None RetryAfterFormat = iota // RetryAfter Format Enum Values
	Seconds
	Date
	Invalid
)

// Test The NoRetries() Functionality
func TestNoRetries(t *testing.T) {
	retryConfig := NoRetries()
	assert.NotNil(t, retryConfig)
	assert.Equal(t, 0, retryConfig.RetryMax)
	assert.NotNil(t, retryConfig.CheckRetry)
	result, err := retryConfig.CheckRetry(context.TODO(), nil, nil)
	assert.False(t, result)
	assert.Nil(t, err)
	assert.NotNil(t, retryConfig.Backoff)
	assert.Equal(t, time.Duration(0), retryConfig.Backoff(1, nil))
	assert.Equal(t, time.Duration(0), retryConfig.Backoff(100, nil))
	assert.Nil(t, retryConfig.RetryAfterMaxDuration)
}

func TestRetryConfigBackoffMax(t *testing.T) {
	linear := v1.BackoffPolicyLinear
	exponential := v1.BackoffPolicyExponential
	tests := []struct {
		name         string
		policy       v1.BackoffPolicyType
		delay        string
		backoffMax   *string
		attempts     []int
		wantBackoffs []time.Duration
		wantErr      bool
	}{
		{
			name:       "linear capped",
			policy:     linear,
			delay:      "PT3S",
			backoffMax: ptr.String("PT10S"),
			attempts:   []int{0, 1, 2, 3, int(^uint(0) >> 1)},
			wantBackoffs: []time.Duration{
				3 * time.Second,
				6 * time.Second,
				9 * time.Second,
				10 * time.Second,
				10 * time.Second,
			},
		},
		{
			name:       "exponential capped",
			policy:     exponential,
			delay:      "PT1S",
			backoffMax: ptr.String("PT10S"),
			attempts:   []int{1, 2, 3, 4, int(^uint(0) >> 1)},
			wantBackoffs: []time.Duration{
				2 * time.Second,
				4 * time.Second,
				8 * time.Second,
				10 * time.Second,
				10 * time.Second,
			},
		},
		{
			name:         "uncapped exponential saturates",
			policy:       exponential,
			delay:        "PT1S",
			attempts:     []int{1, int(^uint(0) >> 1)},
			wantBackoffs: []time.Duration{2 * time.Second, maxBackoffDuration},
		},
		{
			name:         "uncapped linear saturates",
			policy:       linear,
			delay:        "PT1S",
			attempts:     []int{0, int(^uint(0) >> 1)},
			wantBackoffs: []time.Duration{time.Second, maxBackoffDuration},
		},
		{
			name:         "large linear delay saturates",
			policy:       linear,
			delay:        "P293Y",
			attempts:     []int{1},
			wantBackoffs: []time.Duration{maxBackoffDuration},
		},
		{
			name:         "large exponential delay saturates",
			policy:       exponential,
			delay:        "P293Y",
			attempts:     []int{0},
			wantBackoffs: []time.Duration{maxBackoffDuration},
		},
		{
			name:         "large linear maximum saturates",
			policy:       linear,
			delay:        "P292Y",
			backoffMax:   ptr.String("P293Y"),
			attempts:     []int{2},
			wantBackoffs: []time.Duration{maxBackoffDuration},
		},
		{
			name:         "large exponential maximum saturates",
			policy:       exponential,
			delay:        "P292Y",
			backoffMax:   ptr.String("P293Y"),
			attempts:     []int{1},
			wantBackoffs: []time.Duration{maxBackoffDuration},
		},
		{
			name:       "zero maximum rejected",
			policy:     exponential,
			delay:      "PT1S",
			backoffMax: ptr.String("PT0S"),
			wantErr:    true,
		},
		{
			name:       "negative maximum rejected",
			policy:     exponential,
			delay:      "PT1S",
			backoffMax: ptr.String("-PT1S"),
			wantErr:    true,
		},
		{
			name:       "invalid maximum rejected",
			policy:     exponential,
			delay:      "PT1S",
			backoffMax: ptr.String("invalid"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := RetryConfigFromDeliverySpec(v1.DeliverySpec{
				BackoffPolicy: &tt.policy,
				BackoffDelay:  &tt.delay,
				BackoffMax:    tt.backoffMax,
			})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.backoffMax, config.BackoffMax)
			for i, attempt := range tt.attempts {
				assert.Equal(t, tt.wantBackoffs[i], config.Backoff(attempt, nil))
			}
		})
	}
}

func TestDoWithRetriesBackoffMax(t *testing.T) {
	tests := []struct {
		name          string
		policy        v1.BackoffPolicyType
		delay         string
		backoffMax    *string
		retryAfter    string
		retryAfterMax *string
		wantTimes     []time.Duration
	}{
		{
			name:       "exponential backoff is capped",
			policy:     v1.BackoffPolicyExponential,
			delay:      "PT1S",
			backoffMax: ptr.String("PT2S"),
			wantTimes:  []time.Duration{0, time.Second, 3 * time.Second, 5 * time.Second, 7 * time.Second},
		},
		{
			name:      "exponential backoff without a maximum",
			policy:    v1.BackoffPolicyExponential,
			delay:     "PT1S",
			wantTimes: []time.Duration{0, time.Second, 3 * time.Second, 7 * time.Second, 15 * time.Second},
		},
		{
			name:       "maximum below the initial delay",
			policy:     v1.BackoffPolicyExponential,
			delay:      "PT3S",
			backoffMax: ptr.String("PT2S"),
			wantTimes:  []time.Duration{0, 2 * time.Second, 4 * time.Second, 6 * time.Second, 8 * time.Second},
		},
		{
			name:          "retry after can exceed the backoff maximum",
			policy:        v1.BackoffPolicyExponential,
			delay:         "PT1S",
			backoffMax:    ptr.String("PT2S"),
			retryAfter:    "3",
			retryAfterMax: ptr.String("PT5S"),
			wantTimes:     []time.Duration{0, 3 * time.Second, 6 * time.Second, 9 * time.Second, 12 * time.Second},
		},
		{
			name:      "linear backoff waits one delay before the first retry",
			policy:    v1.BackoffPolicyLinear,
			delay:     "PT1S",
			wantTimes: []time.Duration{0, time.Second, 3 * time.Second, 6 * time.Second, 10 * time.Second},
		},
		{
			name:       "linear backoff is capped",
			policy:     v1.BackoffPolicyLinear,
			delay:      "PT1S",
			backoffMax: ptr.String("PT2S"),
			wantTimes:  []time.Duration{0, time.Second, 3 * time.Second, 5 * time.Second, 7 * time.Second},
		},
		{
			name:       "linear maximum below the initial delay",
			policy:     v1.BackoffPolicyLinear,
			delay:      "PT3S",
			backoffMax: ptr.String("PT2S"),
			wantTimes:  []time.Duration{0, 2 * time.Second, 4 * time.Second, 6 * time.Second, 8 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				config, err := RetryConfigFromDeliverySpec(v1.DeliverySpec{
					Retry:         ptr.Int32(4),
					BackoffPolicy: &tt.policy,
					BackoffDelay:  &tt.delay,
					BackoffMax:    tt.backoffMax,
					RetryAfterMax: tt.retryAfterMax,
				})
				require.NoError(t, err)

				start := time.Now()
				var sentAt []time.Duration
				// Only HTTP responses are faked; the retry loop uses its real timers.
				c := client{Client: http.Client{Transport: retryRoundTripperFunc(func(req *http.Request) (*http.Response, error) {
					sentAt = append(sentAt, time.Since(start))
					resp := &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       http.NoBody,
						Request:    req,
					}
					if len(sentAt) <= 4 {
						resp.StatusCode = http.StatusServiceUnavailable
						if tt.retryAfter != "" {
							resp.Header.Set("Retry-After", tt.retryAfter)
						}
					}
					return resp, nil
				})}}
				req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://subscriber.example.com", nil)
				require.NoError(t, err)

				resp, err := c.DoWithRetries(req, &config)
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusNoContent, resp.StatusCode)
				assert.Equal(t, tt.wantTimes, sentAt)
			})
		})
	}
}

type retryRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f retryRoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSaturatingPeriodDuration(t *testing.T) {
	representable := period.MustParse("P292Y")
	want, _ := representable.Duration()
	assert.NotEqual(t, maxBackoffDuration, want)
	assert.Equal(t, want, saturatingPeriodDuration(representable))
	assert.Equal(t, maxBackoffDuration, saturatingPeriodDuration(period.MustParse("P293Y")))
}

func TestBackoffMaxDoesNotCapRetryAfter(t *testing.T) {
	policy := v1.BackoffPolicyExponential
	config, err := RetryConfigFromDeliverySpec(v1.DeliverySpec{
		BackoffPolicy: &policy,
		BackoffDelay:  ptr.String("PT1S"),
		BackoffMax:    ptr.String("PT10S"),
		RetryAfterMax: ptr.String("PT20S"),
	})
	assert.NoError(t, err)

	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Retry-After": []string{"30"}},
	}
	assert.Equal(t, 20*time.Second, generateBackoffFn(&config)(0, 0, 20, resp))
}

// Test The RetryConfigFromDeliverySpec() Functionality
func TestRetryConfigFromDeliverySpec(t *testing.T) {
	const retry = 5
	validISO8601DurationString := "PT30S"
	invalidISO8601DurationString := "FOO"

	testcases := []struct {
		name                     string
		backoffPolicy            v1.BackoffPolicyType
		backoffDelay             string
		timeout                  *string
		retryAfterMax            *string
		expectedBackoffDurations []time.Duration
		wantErr                  bool
	}{{
		name:          "Successful Linear Backoff 2500ms, 5 retries",
		backoffPolicy: v1.BackoffPolicyLinear,
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
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT1.5S",
		expectedBackoffDurations: []time.Duration{
			1500 * time.Millisecond,
			3 * time.Second,
			6 * time.Second,
			12 * time.Second,
			24 * time.Second,
		},
	}, {
		name:          "Successful Exponential Backoff 500ms, 5 retries",
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		expectedBackoffDurations: []time.Duration{
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
		},
	}, {
		name:          "Invalid Backoff Delay",
		backoffPolicy: v1.BackoffPolicyLinear,
		backoffDelay:  "FOO",
		wantErr:       true,
	}, {
		name:          "Valid Timeout",
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		timeout:       &validISO8601DurationString,
		expectedBackoffDurations: []time.Duration{
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
		},
	}, {
		name:          "Invalid Timeout",
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		timeout:       &invalidISO8601DurationString,
		wantErr:       true,
	}, {
		name:          "Valid RetryAfterMax",
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		retryAfterMax: &validISO8601DurationString,
		expectedBackoffDurations: []time.Duration{
			500 * time.Millisecond,
			1 * time.Second,
			2 * time.Second,
			4 * time.Second,
			8 * time.Second,
		},
	}, {
		name:          "Invalid RetryAfterMax",
		backoffPolicy: v1.BackoffPolicyExponential,
		backoffDelay:  "PT0.5S",
		retryAfterMax: &invalidISO8601DurationString,
		wantErr:       true,
	}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {

			// Create The DeliverySpec To Test
			deliverySpec := v1.DeliverySpec{
				DeadLetterSink: nil,
				Retry:          ptr.Int32(retry),
				BackoffPolicy:  &tc.backoffPolicy,
				BackoffDelay:   &tc.backoffDelay,
				Timeout:        tc.timeout,
				RetryAfterMax:  tc.retryAfterMax,
			}

			// Create the RetryConfig from the deliverySpec
			retryConfig, err := RetryConfigFromDeliverySpec(deliverySpec)
			assert.Equal(t, tc.wantErr, err != nil)

			// If successful then validate the retryConfig (Max & Backoff calculations).
			if err == nil {
				assert.Equal(t, retry, retryConfig.RetryMax)
				if tc.timeout != nil && *tc.timeout != "" {
					expectedTimeoutPeriod, _ := period.Parse(*tc.timeout)
					expectedTimeoutDuration, _ := expectedTimeoutPeriod.Duration()
					assert.Equal(t, expectedTimeoutDuration, retryConfig.RequestTimeout)
				}

				if tc.retryAfterMax != nil && *tc.retryAfterMax != "" {
					expectedMaxPeriod, _ := period.Parse(*tc.retryAfterMax)
					expectedMaxDuration, _ := expectedMaxPeriod.Duration()
					assert.Equal(t, expectedMaxDuration, *retryConfig.RetryAfterMaxDuration)
				} else {
					assert.Nil(t, retryConfig.RetryAfterMaxDuration)
				}

				for i := 0; i < retry; i++ {
					expectedBackoffDuration := tc.expectedBackoffDurations[i]
					actualBackoffDuration := retryConfig.Backoff(i, nil)
					assert.Equal(t, expectedBackoffDuration, actualBackoffDuration)
				}
			}
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
			name:     "Http StatusCode 408",
			response: &http.Response{StatusCode: http.StatusRequestTimeout},
			result:   true,
		},
		{
			name:     "Http StatusCode 409",
			response: &http.Response{StatusCode: http.StatusConflict},
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
	linear := v1.BackoffPolicyLinear
	tests := []struct {
		name    string
		spec    v1.DeliverySpec
		wantErr bool
	}{{
		name: "full delivery",
		spec: v1.DeliverySpec{
			Retry:         pointer.Int32(10),
			BackoffPolicy: &linear,
			BackoffDelay:  pointer.String("PT1S"),
			Timeout:       pointer.String("PT10S"),
		},
	}, {
		name: "only retry",
		spec: v1.DeliverySpec{
			Retry:         pointer.Int32(10),
			BackoffPolicy: &linear,
		},
	}, {
		name: "delay not ISO8601",
		spec: v1.DeliverySpec{
			Retry:         pointer.Int32(10),
			BackoffDelay:  pointer.String("PP1"),
			BackoffPolicy: &linear,
		},
		wantErr: true,
	}, {
		name: "timeout not ISO8601",
		spec: v1.DeliverySpec{
			Retry:   pointer.Int32(10),
			Timeout: pointer.String("PP1"),
		},
		wantErr: true,
	}}

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
		response.Header["Retry-After"] = []string{strconv.Itoa(int(duration.Seconds()))}
	case Date:
		response.Header["Retry-After"] = []string{time.Now().Add(duration).Format(time.RFC850)} // RFC850 Drops Millis
	case Invalid:
		response.Header["Retry-After"] = []string{"FOO"}
	default:
		assert.Fail(t, "TestCase with unsupported ResponseFormat '%v'", format)
	}

	// Return The HTTP Response
	return response
}
