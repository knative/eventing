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
	nethttp "net/http"
	"sync"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

const (
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
)

type holder struct {
	clientMutex    sync.Mutex
	connectionArgs *ConnectionArgs
	client         **nethttp.Client
}

var clientHolder = holder{}

// The used HTTP client is a singleton, so the same http client is reused across all the application.
// If connection args is modified, client is cleaned and a new one is created.
func getClient() *nethttp.Client {
	clientHolder.clientMutex.Lock()
	defer clientHolder.clientMutex.Unlock()

	if clientHolder.client == nil {
		// Add connection options to the default transport.
		var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()
		clientHolder.connectionArgs.configureTransport(base)
		c := &nethttp.Client{
			// Add output tracing.
			Transport: &ochttp.Transport{
				Base:        base,
				Propagation: tracecontextb3.TraceContextEgress,
			},
		}
		clientHolder.client = &c
	}

	return *clientHolder.client
}

// ConfigureConnectionArgs configures the new connection args.
// The existing client won't be affected, but a new one will be created.
// Use sparingly, because it might lead to creating a lot of clients, none of them sharing their connection pool!
func ConfigureConnectionArgs(ca *ConnectionArgs) {
	clientHolder.clientMutex.Lock()
	defer clientHolder.clientMutex.Unlock()

	// Check if same config
	if clientHolder.connectionArgs != nil &&
		ca != nil &&
		ca.MaxIdleConns == clientHolder.connectionArgs.MaxIdleConns &&
		ca.MaxIdleConnsPerHost == clientHolder.connectionArgs.MaxIdleConnsPerHost {
		return
	}

	if clientHolder.client != nil {
		// Let's try to clean up a bit the existing client
		// Note: this won't remove it nor close it
		(*clientHolder.client).CloseIdleConnections()

		// Setting client to nil
		clientHolder.client = nil
	}

	clientHolder.connectionArgs = ca
}

// ConnectionArgs allow to configure connection parameters to the underlying
// HTTP Client transport.
type ConnectionArgs struct {
	// MaxIdleConns refers to the max idle connections, as in net/http/transport.
	MaxIdleConns int
	// MaxIdleConnsPerHost refers to the max idle connections per host, as in net/http/transport.
	MaxIdleConnsPerHost int
}

func (ca *ConnectionArgs) configureTransport(transport *nethttp.Transport) {
	if ca == nil {
		return
	}
	transport.MaxIdleConns = ca.MaxIdleConns
	transport.MaxIdleConnsPerHost = ca.MaxIdleConnsPerHost
}
