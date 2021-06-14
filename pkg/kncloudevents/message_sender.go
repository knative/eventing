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
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

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
