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
	nethttp "net/http"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
)

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
			Propagation: &tracecontext.HTTPFormat{},
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
