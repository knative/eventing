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
	"fmt"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const RetryAfterHeader = "Retry-After"

// HTTPMessageSender is a wrapper for an http client that can send cloudevents.Request with retries
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

func (s *HTTPMessageSender) SendWithRetries(req *nethttp.Request, config *RetryConfig) (*nethttp.Response, error) {
	if config == nil {
		return s.Send(req)
	}

	client := s.Client
	if config.RequestTimeout != 0 {
		client = &nethttp.Client{
			Transport:     client.Transport,
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
			Timeout:       config.RequestTimeout,
		}
	}

	retryableClient := retryablehttp.Client{
		HTTPClient:   client,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     config.RetryMax,
		CheckRetry:   retryablehttp.CheckRetry(config.CheckRetry),
		Backoff:      generateBackoffFn(config),
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

// generateBackoffFunction returns a valid retryablehttp.Backoff implementation which
// wraps the provided RetryConfig.Backoff implementation with optional "Retry-After"
// header support.
func generateBackoffFn(config *RetryConfig) retryablehttp.Backoff {
	return func(_, _ time.Duration, attemptNum int, resp *nethttp.Response) time.Duration {

		// If Response is 429 or 503 Then Parse Any Retry-After Header Durations & Enforce Optional MaxDuration
		var retryAfterDuration time.Duration
		if resp != nil && (resp.StatusCode == nethttp.StatusTooManyRequests || resp.StatusCode == nethttp.StatusServiceUnavailable) {
			if config.RetryAfterEnabled {
				retryAfterDuration, _ = parseRetryAfterDuration(resp)
				if config.RetryAfterMaxDuration > 0 && config.RetryAfterMaxDuration < retryAfterDuration {
					retryAfterDuration = config.RetryAfterMaxDuration
				}
			}
		}

		// Calculate The RetryConfig Backoff Duration
		backoffDuration := config.Backoff(attemptNum, resp)

		// Return The Larger Of The Two Backoff Durations
		if retryAfterDuration > backoffDuration {
			return retryAfterDuration
		} else {
			return backoffDuration
		}
	}
}

// parseRetryAfterDuration returns a Duration expressing the amount of time
// requested to wait by a Retry-After header, or none if not present or invalid.
// According to the spec (https://tools.ietf.org/html/rfc7231#section-7.1.3)
// the Retry-After Header's value can be one of an HTTP-date or delay-seconds,
// both of which are supported here.
func parseRetryAfterDuration(resp *nethttp.Response) (time.Duration, error) {
	var retryAfterDuration time.Duration
	if resp != nil && resp.Header != nil {
		retryAfterString := resp.Header.Get(RetryAfterHeader)
		if len(retryAfterString) > 0 {
			if retryAfterInt, parseIntErr := strconv.ParseInt(retryAfterString, 10, 64); parseIntErr == nil {
				retryAfterDuration = time.Duration(retryAfterInt) * time.Second
			} else {
				retryAfterTime, parseTimeErr := nethttp.ParseTime(retryAfterString) // Supports http.TimeFormat, time.RFC850 & time.ANSIC
				if parseTimeErr != nil {
					return retryAfterDuration, fmt.Errorf("failed to parse Retry-After header: ParseInt Error = %v, ParseTime Error = %v", parseIntErr, parseTimeErr)
				}
				retryAfterDuration = time.Until(retryAfterTime)
			}
		}
	}
	return retryAfterDuration, nil
}
