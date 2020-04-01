/*
 * Copyright 2019 The Knative Authors
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
	nethttp "net/http"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"github.com/cloudevents/sdk-go/v1/cloudevents/client"
	"github.com/cloudevents/sdk-go/v1/cloudevents/transport/http"
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

func (ca *ConnectionArgs) NewDefaultHTTPTransport() *nethttp.Transport {
	var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()
	ca.ConfigureTransport(base)
	return base
}

func NewDefaultClient(target ...string) (cloudevents.Client, error) {
	tOpts := []http.Option{
		cloudevents.WithBinaryEncoding(),
	}
	if len(target) > 0 && target[0] != "" {
		tOpts = append(tOpts, cloudevents.WithTarget(target[0]))
	}

	// Make an http transport for the CloudEvents client.
	t, err := cloudevents.NewHTTPTransport(tOpts...)
	if err != nil {
		return nil, err
	}
	return NewDefaultHTTPClient(t)
}

// NewDefaultHTTPClient creates a new client from an HTTP transport.
func NewDefaultHTTPClient(t *cloudevents.HTTPTransport, opts ...client.Option) (cloudevents.Client, error) {
	if opts == nil {
		opts = make([]client.Option, 0, 2)
	}
	opts = append(opts, cloudevents.WithUUIDs(), cloudevents.WithTimeNow())

	// Use the transport to make a new CloudEvents client.
	c, err := cloudevents.NewClient(t, opts...)

	if err != nil {
		return nil, err
	}
	return c, nil
}
