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
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

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
