/*
 * Copyright 2020 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package kncloudevents

import (
	"context"
	"fmt"
	"math"
	nethttp "net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/rickb777/date/period"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

var noRetries = RetryConfig{
	RetryMax: 0,
	CheckRetry: func(ctx context.Context, resp *nethttp.Response, err error) (bool, error) {
		return false, nil
	},
	Backoff: func(attemptNum int, resp *nethttp.Response) time.Duration {
		return 0
	},
}

type HTTPMessageSender struct {
	Client *nethttp.Client
	Target string
}

// Deprecated: Don't use this anymore, now it has the same effect of NewHTTPMessageSenderWithTarget
// If you need to modify the connection args, use ConfigureConnectionArgs sparingly.
func NewHTTPMessageSender(ca *ConnectionArgs, target string) (*HTTPMessageSender, error) {
	return NewHTTPMessageSenderWithTarget(target)
}

func NewHTTPMessageSenderWithTarget(target string) (*HTTPMessageSender, error) {
	return &HTTPMessageSender{Client: getClient(), Target: target}, nil
}

func (s *HTTPMessageSender) NewCloudEventRequest(ctx context.Context) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", s.Target, nil)
}

func (s *HTTPMessageSender) NewCloudEventRequestWithTarget(ctx context.Context, target string) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", target, nil)
}

func (s *HTTPMessageSender) Send(req *nethttp.Request) (*nethttp.Response, error) {
	return s.Client.Do(req)
}

// CheckRetry specifies a policy for handling retries. It is called
// following each request with the response and error values returned by
// the http.Client. If CheckRetry returns false, the Client stops retrying
// and returns the response to the caller. If CheckRetry returns an error,
// that error value is returned in lieu of the error from the request. The
// Client will close any response body when retrying, but if the retry is
// aborted it is up to the CheckRetry callback to properly close any
// response body before returning.
type CheckRetry func(ctx context.Context, resp *nethttp.Response, err error) (bool, error)

// Backoff specifies a policy for how long to wait between retries.
// It is called after a failing request to determine the amount of time
// that should pass before trying again.
type Backoff func(attemptNum int, resp *nethttp.Response) time.Duration

type RetryConfig struct {
	// Maximum number of retries
	RetryMax int
	// These next two variables are just copied from the original DeliverySpec so
	// we can detect if anything has changed. We can not do that with the CheckRetry
	// Backoff (at least not easily).
	BackoffDelay  *string
	BackoffPolicy *duckv1.BackoffPolicyType

	CheckRetry CheckRetry
	Backoff    Backoff
}

func (s *HTTPMessageSender) SendWithRetries(req *nethttp.Request, config *RetryConfig) (*nethttp.Response, error) {
	if config == nil {
		return s.Send(req)
	}

	retryableClient := retryablehttp.Client{
		HTTPClient:   s.Client,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     config.RetryMax,
		CheckRetry:   retryablehttp.CheckRetry(config.CheckRetry),
		Backoff: func(_, _ time.Duration, attemptNum int, resp *nethttp.Response) time.Duration {
			return config.Backoff(attemptNum, resp)
		},
		ErrorHandler: func(resp *nethttp.Response, err error, numTries int) (*nethttp.Response, error) {
			return resp, err
		},
	}

	retryableReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	return retryableClient.Do(retryableReq)
}

func NoRetries() RetryConfig {
	return noRetries
}

func RetryConfigFromDeliverySpec(spec duckv1.DeliverySpec) (RetryConfig, error) {

	retryConfig := NoRetries()

	retryConfig.CheckRetry = checkRetry

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
		case duckv1.BackoffPolicyExponential:
			retryConfig.Backoff = func(attemptNum int, resp *nethttp.Response) time.Duration {
				return delayDuration * time.Duration(math.Exp2(float64(attemptNum)))
			}
		case duckv1.BackoffPolicyLinear:
			retryConfig.Backoff = func(attemptNum int, resp *nethttp.Response) time.Duration {
				return delayDuration * time.Duration(attemptNum)
			}
		}
	}

	return retryConfig, nil
}

func checkRetry(_ context.Context, resp *nethttp.Response, err error) (bool, error) {
	return !(resp != nil && resp.StatusCode < 300), err
}
