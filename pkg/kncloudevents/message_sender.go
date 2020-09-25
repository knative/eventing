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
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
)

const (
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
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

// ConnectionArgs allow to configure connection parameters to the underlying
// HTTP Client transport.
type ConnectionArgs struct {
	// MaxIdleConns refers to the max idle connections, as in net/http/transport.
	MaxIdleConns int
	// MaxIdleConnsPerHost refers to the max idle connections per host, as in net/http/transport.
	MaxIdleConnsPerHost int
}

func (ca *ConnectionArgs) ConfigureTransport(transport *nethttp.Transport) {
	if ca == nil {
		return
	}
	transport.MaxIdleConns = ca.MaxIdleConns
	transport.MaxIdleConnsPerHost = ca.MaxIdleConnsPerHost
}

type HttpMessageSender struct {
	Client *nethttp.Client
	Target string
}

func NewHttpMessageSender(connectionArgs *ConnectionArgs, target string) (*HttpMessageSender, error) {
	// Add connection options to the default transport.
	var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()
	connectionArgs.ConfigureTransport(base)
	// Add output tracing.
	client := &nethttp.Client{
		Transport: &ochttp.Transport{
			Base:        base,
			Propagation: tracecontextb3.TraceContextEgress,
		},
	}

	return &HttpMessageSender{Client: client, Target: target}, nil
}

func (s *HttpMessageSender) NewCloudEventRequest(ctx context.Context) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", s.Target, nil)
}

func (s *HttpMessageSender) NewCloudEventRequestWithTarget(ctx context.Context, target string) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", target, nil)
}

func (s *HttpMessageSender) Send(req *nethttp.Request) (*nethttp.Response, error) {
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

	CheckRetry CheckRetry

	Backoff Backoff
}

func (s *HttpMessageSender) SendWithRetries(req *nethttp.Request, config *RetryConfig) (*nethttp.Response, error) {
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

	return retryableClient.Do(&retryablehttp.Request{Request: req})
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
				return delayDuration
			}
		}
	}

	return retryConfig, nil
}

func checkRetry(_ context.Context, resp *nethttp.Response, _ error) (bool, error) {
	return resp != nil && resp.StatusCode >= 300, nil
}
