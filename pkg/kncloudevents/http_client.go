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

	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

type legacyHolder struct {
	clientMutex    sync.Mutex
	connectionArgs *ConnectionArgs
	client         **nethttp.Client
}

var legacyClientHolder = legacyHolder{}

// The used HTTP client is a singleton, so the same http client is reused across all the application.
// If connection args is modified, client is cleaned and a new one is created.
func getClient() *nethttp.Client {
	legacyClientHolder.clientMutex.Lock()
	defer legacyClientHolder.clientMutex.Unlock()

	if legacyClientHolder.client == nil {
		// Add connection options to the default transport.
		var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()
		legacyClientHolder.connectionArgs.configureTransport(base)
		c := &nethttp.Client{
			// Add output tracing.
			Transport: &ochttp.Transport{
				Base:        base,
				Propagation: tracecontextb3.TraceContextEgress,
			},
		}
		legacyClientHolder.client = &c
	}

	return *legacyClientHolder.client
}

// ConfigureConnectionArgs configures the new connection args.
// The existing client won't be affected, but a new one will be created.
// Use sparingly, because it might lead to creating a lot of clients, none of them sharing their connection pool!
func configureConnectionArgsOldClient(ca *ConnectionArgs) {
	legacyClientHolder.clientMutex.Lock()
	defer legacyClientHolder.clientMutex.Unlock()

	// Check if same config
	if legacyClientHolder.connectionArgs != nil &&
		ca != nil &&
		ca.MaxIdleConns == legacyClientHolder.connectionArgs.MaxIdleConns &&
		ca.MaxIdleConnsPerHost == legacyClientHolder.connectionArgs.MaxIdleConnsPerHost {
		return
	}

	if legacyClientHolder.client != nil {
		// Let's try to clean up a bit the existing client
		// Note: this won't remove it nor close it
		(*legacyClientHolder.client).CloseIdleConnections()

		// Setting client to nil
		legacyClientHolder.client = nil
	}

	legacyClientHolder.connectionArgs = ca
}
