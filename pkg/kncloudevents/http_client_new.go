/*
Copyright 2023 The Knative Authors

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
	"fmt"
	nethttp "net/http"
	"sync"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"knative.dev/eventing/pkg/eventingtls"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

const (
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 30 * time.Second
)

var (
	clients clientsHolder
)

type clientsHolder struct {
	mu             sync.Mutex
	clients        map[string]*nethttp.Client
	connectionArgs *ConnectionArgs
}

func init() {
	clients = clientsHolder{
		clients: make(map[string]*nethttp.Client),
	}
}

func getClientForAddressable(addressable duckv1.Addressable) (*nethttp.Client, error) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	clientKey := addressable.URL.String()

	client, ok := clients.clients[clientKey]
	if !ok {
		newClient, err := createNewClient(addressable)
		if err != nil {
			return nil, fmt.Errorf("failed to create new client for addressable: %w", err)
		}

		clients.clients[clientKey] = newClient

		client = newClient
	}

	return client, nil
}

func createNewClient(addressable duckv1.Addressable) (*nethttp.Client, error) {
	var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()

	if addressable.CACerts != nil && *addressable.CACerts != "" {
		var err error

		clientConfig := eventingtls.NewDefaultClientConfig()
		clientConfig.CACerts = addressable.CACerts

		base.TLSClientConfig, err = eventingtls.GetTLSClientConfig(clientConfig)
		if err != nil {
			return nil, err
		}
	}

	clients.connectionArgs.configureTransport(base)
	client := &nethttp.Client{
		// Add output tracing.
		Transport: &ochttp.Transport{
			Base:        base,
			Propagation: tracecontextb3.TraceContextEgress,
		},
	}

	return client, nil
}

func AddOrUpdateAddressableHandler(addressable duckv1.Addressable) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	clientKey := addressable.URL.String()

	client, err := createNewClient(addressable)
	if err != nil {
		fmt.Printf("failed to create new client: %v", err)
		return
	}
	clients.clients[clientKey] = client
}

func DeleteAddressableHandler(addressable duckv1.Addressable) {
	clients.mu.Lock()
	defer clients.mu.Unlock()

	clientKey := addressable.URL.String()

	delete(clients.clients, clientKey)
}

// ConfigureConnectionArgs configures the new connection args.
// Use sparingly, because it might lead to creating a lot of clients, none of them sharing their connection pool!
func ConfigureConnectionArgs(ca *ConnectionArgs) {
	configureConnectionArgsOldClient(ca) //also configure the connection args of the old client

	clients.mu.Lock()
	defer clients.mu.Unlock()

	// Check if same config
	if clients.connectionArgs != nil &&
		ca != nil &&
		ca.MaxIdleConns == clients.connectionArgs.MaxIdleConns &&
		ca.MaxIdleConnsPerHost == clients.connectionArgs.MaxIdleConnsPerHost {
		return
	}

	if len(clients.clients) > 0 {
		// Let's try to clean up a bit the existing clients
		// Note: this won't remove it nor close it
		for _, client := range clients.clients {
			client.CloseIdleConnections()
		}

		// Resetting clients
		clients.clients = make(map[string]*nethttp.Client)
	}

	clients.connectionArgs = ca
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
