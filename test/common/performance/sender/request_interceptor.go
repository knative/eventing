/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sender

import "net/http"

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
)

type requestInterceptor struct {
	before       func(*http.Request)
	after        func(*http.Request, *http.Response, error)
	roundTripper http.RoundTripper
}

func NewRequestInterceptor(before func(*http.Request), after func(*http.Request, *http.Response, error)) *requestInterceptor {
	transport := http.DefaultTransport
	t := transport.(*http.Transport)
	t.MaxIdleConns = defaultMaxIdleConnections
	t.MaxIdleConnsPerHost = defaultMaxIdleConnectionsPerHost
	return &requestInterceptor{
		before:       before,
		after:        after,
		roundTripper: t,
	}
}

func (r requestInterceptor) RoundTrip(request *http.Request) (*http.Response, error) {
	if r.before != nil {
		r.before(request)
	}

	var res *http.Response
	var err error
	if r.roundTripper == nil {
		res, err = http.DefaultTransport.RoundTrip(request)
	} else {
		res, err = r.roundTripper.RoundTrip(request)
	}

	if r.after != nil {
		r.after(request, res, err)
	}
	return res, err
}
