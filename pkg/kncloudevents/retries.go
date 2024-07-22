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
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rickb777/date/period"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
)

var noRetries = RetryConfig{
	RetryMax: 0,
	CheckRetry: func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		return false, nil
	},
	Backoff: func(attemptNum int, resp *http.Response) time.Duration {
		return 0
	},
}

// CheckRetry specifies a policy for handling retries. It is called
// following each request with the response and error values returned by
// the http.Client. If CheckRetry returns false, the Client stops retrying
// and returns the response to the caller. If CheckRetry returns an error,
// that error value is returned in lieu of the error from the request. The
// Client will close any response body when retrying, but if the retry is
// aborted it is up to the CheckRetry callback to properly close any
// response body before returning.
type CheckRetry func(ctx context.Context, resp *http.Response, err error) (bool, error)

// Backoff specifies a policy for how long to wait between retries.
// It is called after a failing request to determine the amount of time
// that should pass before trying again.
type Backoff func(attemptNum int, resp *http.Response) time.Duration

type RetryConfig struct {
	// Maximum number of retries
	RetryMax int
	// These next two variables are just copied from the original DeliverySpec so
	// we can detect if anything has changed. We can not do that with the CheckRetry
	// Backoff (at least not easily).
	BackoffDelay  *string
	BackoffPolicy *v1.BackoffPolicyType

	CheckRetry CheckRetry
	Backoff    Backoff

	// RequestTimeout represents the timeout of the single request
	RequestTimeout time.Duration

	// RetryAfterMaxDuration represents an optional override for the maximum
	// value allowed for "Retry-After" headers in 429 / 503 responses.  A nil
	// value indicates no maximum override.  A value of "0" indicates "Retry-After"
	// headers are to be ignored.
	RetryAfterMaxDuration *time.Duration
}

func NoRetries() RetryConfig {
	return noRetries
}

func RetryConfigFromDeliverySpec(spec v1.DeliverySpec) (RetryConfig, error) {

	retryConfig := NoRetries()

	retryConfig.CheckRetry = SelectiveRetry

	if spec.Retry != nil {
		retryConfig.RetryMax = int(*spec.Retry)
	}
	retryConfig.BackoffPolicy = spec.BackoffPolicy
	retryConfig.BackoffDelay = spec.BackoffDelay

	if spec.BackoffPolicy != nil && spec.BackoffDelay != nil {

		delay, err := period.Parse(*spec.BackoffDelay)
		if err != nil {
			return retryConfig, fmt.Errorf("failed to parse Spec.BackoffDelay: %w", err)
		}

		delayDuration, _ := delay.Duration()
		switch *spec.BackoffPolicy {
		case v1.BackoffPolicyExponential:
			retryConfig.Backoff = func(attemptNum int, resp *http.Response) time.Duration {
				return delayDuration * time.Duration(math.Exp2(float64(attemptNum)))
			}
		case v1.BackoffPolicyLinear:
			retryConfig.Backoff = func(attemptNum int, resp *http.Response) time.Duration {
				return delayDuration * time.Duration(attemptNum)
			}
		}
	}

	if spec.Timeout != nil {
		timeout, err := period.Parse(*spec.Timeout)
		if err != nil {
			return retryConfig, fmt.Errorf("failed to parse Spec.Timeout: %w", err)
		}
		retryConfig.RequestTimeout, _ = timeout.Duration()
	}

	if spec.RetryAfterMax != nil {
		maxPeriod, err := period.Parse(*spec.RetryAfterMax)
		if err != nil { // Should never happen based on DeliverySpec validation
			return retryConfig, fmt.Errorf("failed to parse Spec.RetryAfterMax: %w", err)
		}
		maxDuration, _ := maxPeriod.Duration()
		retryConfig.RetryAfterMaxDuration = &maxDuration
	}

	return retryConfig, nil
}

// SelectiveRetry is an alternative function to determine whether to retry based on response
//
// Note - Returning true indicates a retry should occur.  Returning an error will result in that
//
//	error being returned instead of any errors from the Request.
//
// A retry is triggered for:
// * nil responses
// * emitted errors
// * status codes that are 5XX, 404, 408, 409, 429 as well if the statuscode is -1.
func SelectiveRetry(_ context.Context, response *http.Response, err error) (bool, error) {

	// Retry Any Nil HTTP Response
	if response == nil {
		return true, nil
	}

	// Retry Any Errors
	if err != nil {
		return true, nil
	}

	// Extract The StatusCode From The Response & Add To Logger
	statusCode := response.StatusCode

	// Note - Normally we would NOT want to retry 4xx responses, BUT there are a few
	//        known areas of knative-eventing that return codes in this range which
	//        require retries.  Reasons for particular codes are as follows:
	//
	// 404  Although we would ideally not want to retry a permanent "Not Found"
	//      response, a 404 can be returned when a pod is in the process of becoming
	//      ready, so a retry can be a useful thing in this situation.
	// 408  Request Timeout is a good practice to issue a retry.
	// 409  Returned by the E2E tests, so we must retry when "Conflict" is received, or the
	//      tests will fail (see knative.dev/eventing/test/lib/recordevents/receiver/receiver.go)
	// 429  Since retry typically involves a delay (usually an exponential backoff),
	//      retrying after receiving a "Too Many Requests" response is useful.

	if statusCode >= 500 || statusCode == 404 || statusCode == 429 || statusCode == 408 || statusCode == 409 {
		return true, nil
	} else if statusCode >= 300 && statusCode <= 399 {
		return false, nil
	} else if statusCode == -1 {
		return true, nil
	}

	// Do Not Retry 1XX, 2XX, 3XX & Most 4XX StatusCode Responses
	return false, nil
}

// generateBackoffFunction returns a valid retryablehttp.Backoff implementation which
// wraps the provided RetryConfig.Backoff implementation with optional "Retry-After"
// header support.
func generateBackoffFn(config *RetryConfig) retryablehttp.Backoff {
	return func(_, _ time.Duration, attemptNum int, resp *http.Response) time.Duration {

		//
		// NOTE - The following logic will need to be altered slightly once the "delivery-retryafter"
		//        experimental-feature graduates from Alpha/Beta to Stable/GA.  This is according to
		//        plan as described in https://github.com/knative/eventing/issues/5811.
		//
		//        During the Alpha/Beta stages the ability to respect Retry-After headers is "opt-in"
		//        requiring the DeliverySpec.RetryAfterMax to be populated.  The Stable/GA behavior
		//        will be "opt-out" where Retry-After headers are always respected (in the context of
		//        calculating backoff durations for 429 / 503 responses) unless the
		//        DeliverySpec.RetryAfterMax is set to "PT0S".
		//
		//        While this might seem unnecessarily complex, it achieves the following design goals...
		//          - Does not require an explicit "enabled" flag in the DeliverySpec.
		//          - Does not require implementations calling the message_sender to be aware of experimental-features.
		//          - Does not modify existing Knative CRs with arbitrary default "max" values.
		//
		//        The intended behavior of RetryConfig.RetryAfterMaxDuration is as follows...
		//
		//          RetryAfterMaxDuration    Alpha/Beta                              Stable/GA
		//          ---------------------    ----------                              ---------
		//               nil                 Do NOT respect Retry-After headers      Respect Retry-After headers without Max
		//                0                  Do NOT respect Retry-After headers      Do NOT respect Retry-After headers
		//               >0                  Respect Retry-After headers with Max    Respect Retry-After headers with Max
		//

		// If Response is 429 / 503, Then Parse Any Retry-After Header Durations & Enforce Optional MaxDuration
		var retryAfterDuration time.Duration
		// TODO - Remove this check when experimental-feature moves to Stable/GA to convert behavior from opt-in to opt-out
		if config.RetryAfterMaxDuration != nil {
			// TODO - Keep this logic as is (no change required) when experimental-feature is Stable/GA
			if resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) {
				retryAfterDuration = parseRetryAfterDuration(resp)
				if config.RetryAfterMaxDuration != nil && *config.RetryAfterMaxDuration < retryAfterDuration {
					retryAfterDuration = *config.RetryAfterMaxDuration
				}
			}
		}

		// Calculate The RetryConfig Backoff Duration
		backoffDuration := config.Backoff(attemptNum, resp)

		// Return The Larger Of The Two Backoff Durations
		if retryAfterDuration > backoffDuration {
			return retryAfterDuration
		}
		return backoffDuration
	}
}

// parseRetryAfterDuration returns a Duration expressing the amount of time
// requested to wait by a Retry-After header, or 0 if not present or invalid.
// According to the spec (https://tools.ietf.org/html/rfc7231#section-7.1.3)
// the Retry-After Header's value can be one of an HTTP-date or delay-seconds,
// both of which are supported here.
func parseRetryAfterDuration(resp *http.Response) (retryAfterDuration time.Duration) {

	// Return 0 Duration If No Response / Headers
	if resp == nil || resp.Header == nil {
		return
	}

	// Return 0 Duration If No Retry-After Header
	retryAfterString := resp.Header.Get("Retry-After")
	if len(retryAfterString) <= 0 {
		return
	}

	// Attempt To Parse Retry-After Header As Seconds - Return If Successful
	retryAfterInt, parseIntErr := strconv.ParseInt(retryAfterString, 10, 64)
	if parseIntErr == nil {
		return time.Duration(retryAfterInt) * time.Second
	}

	// Attempt To Parse Retry-After Header As Timestamp (RFC850 & ANSIC) - Return If Successful
	retryAfterTime, parseTimeErr := http.ParseTime(retryAfterString)
	if parseTimeErr != nil {
		fmt.Printf("failed to parse Retry-After header: ParseInt Error = %v, ParseTime Error = %v\n", parseIntErr, parseTimeErr)
		return
	}
	return time.Until(retryAfterTime)
}
