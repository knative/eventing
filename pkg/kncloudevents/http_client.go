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
	"context"
	"fmt"
	"net"
	nethttp "net/http"
	"sync"
	"time"

	"go.opencensus.io/plugin/ochttp"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"

	"knative.dev/eventing/pkg/eventingtls"
)

const (
	defaultRetryWaitMin    = 1 * time.Second
	defaultRetryWaitMax    = 30 * time.Second
	defaultCleanupInterval = 5 * time.Minute
)

var (
	clients clientsHolder
)

type clientsHolder struct {
	clientsMu       sync.Mutex
	clients         map[string]*nethttp.Client
	timerMu         sync.Mutex
	connectionArgs  *ConnectionArgs
	cleanupInterval time.Duration
	cancelCleanup   context.CancelFunc
}

func init() {
	ctx, cancel := context.WithCancel(context.Background())
	clients = clientsHolder{
		clients:         make(map[string]*nethttp.Client),
		cancelCleanup:   cancel,
		cleanupInterval: defaultCleanupInterval,
	}
	go cleanupClientsMap(ctx)
}

func getClientForAddressable(cfg eventingtls.ClientConfig, addressable duckv1.Addressable) (*nethttp.Client, error) {
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

	clientKey := addressable.URL.String()

	client, ok := clients.clients[clientKey]
	if !ok {
		newClient, err := createNewClient(cfg, addressable)
		if err != nil {
			return nil, fmt.Errorf("failed to create new client for addressable: %w", err)
		}

		clients.clients[clientKey] = newClient

		client = newClient
	}

	return client, nil
}

//nolint:unparam  // error is always nil
func createNewClient(cfg eventingtls.ClientConfig, addressable duckv1.Addressable) (*nethttp.Client, error) {
	var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()

	if eventingtls.IsHttpsSink(addressable.URL.String()) {
		clientConfig := eventingtls.ClientConfig{
			CACerts:                    addressable.CACerts,
			TrustBundleConfigMapLister: cfg.TrustBundleConfigMapLister,
		}

		base.DialTLSContext = func(ctx context.Context, net, addr string) (net.Conn, error) {
			tlsConfig, err := eventingtls.GetTLSClientConfig(clientConfig)
			if err != nil {
				return nil, err
			}
			return network.DialTLSWithBackOff(ctx, net, addr, tlsConfig)
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

func AddOrUpdateAddressableHandler(cfg eventingtls.ClientConfig, addressable duckv1.Addressable) {
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

	clientKey := addressable.URL.String()

	client, err := createNewClient(cfg, addressable)
	if err != nil {
		fmt.Printf("failed to create new client: %v", err)
		return
	}
	clients.clients[clientKey] = client
}

func DeleteAddressableHandler(addressable duckv1.Addressable) {
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

	clientKey := addressable.URL.String()

	delete(clients.clients, clientKey)
}

// ConfigureConnectionArgs configures the new connection args.
// Use sparingly, because it might lead to creating a lot of clients, none of them sharing their connection pool!
func ConfigureConnectionArgs(ca *ConnectionArgs) {

	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

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
		for _, clientEntry := range clients.clients {
			clientEntry.CloseIdleConnections()
		}

		// Resetting clients
		clients.clients = make(map[string]*nethttp.Client)
	}

	clients.connectionArgs = ca
}

// SetClientCleanupInterval sets the interval before the clients map is re-checked for expired entries.
// forceRestart will force the loop to restart with the new interval, cancelling the current iteration.
func SetClientCleanupInterval(cleanupInterval time.Duration, forceRestart bool) {
	clients.timerMu.Lock()
	clients.cleanupInterval = cleanupInterval
	clients.timerMu.Unlock()
	if forceRestart {
		clients.clientsMu.Lock()
		clients.cancelCleanup()
		ctx, cancel := context.WithCancel(context.Background())
		clients.cancelCleanup = cancel
		clients.clientsMu.Unlock()
		go cleanupClientsMap(ctx)
	}
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

func cleanupClientsMap(ctx context.Context) {
	for {
		clients.timerMu.Lock()
		t := time.NewTimer(clients.cleanupInterval)
		clients.timerMu.Unlock()
		select {
		case <-ctx.Done():
			clients.timerMu.Lock()
			t.Stop()
			clients.timerMu.Unlock()
			return
		case <-t.C:
			clients.clientsMu.Lock()
			for _, cme := range clients.clients {
				cme.CloseIdleConnections()
			}
			clients.clientsMu.Unlock()
		}
	}
}
