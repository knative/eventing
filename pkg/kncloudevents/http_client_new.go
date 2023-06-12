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
	nethttp "net/http"
	"sync"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"knative.dev/eventing/pkg/eventingtls"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

const (
	defaultRetryWaitMin    = 1 * time.Second
	defaultRetryWaitMax    = 30 * time.Second
	defaultClientsTTL      = 30 * time.Minute
	defaultRecheckInterval = 5 * time.Minute
)

var (
	clients clientsHolder
)

type clientsHolder struct {
	clientsMu       sync.Mutex
	clients         map[string]*clientsMapEntry
	timerMu         sync.Mutex
	connectionArgs  *ConnectionArgs
	ttl             time.Duration
	recheckInterval time.Duration
	cancelCleanup   context.CancelFunc
}

type clientsMapEntry struct {
	client       *nethttp.Client
	lastAccessed time.Time
}

func init() {
	ctx, cancel := context.WithCancel(context.Background())
	clients = clientsHolder{
		clients:         make(map[string]*clientsMapEntry),
		cancelCleanup:   cancel,
		ttl:             defaultClientsTTL,
		recheckInterval: defaultRecheckInterval,
	}
	go cleanupClientsMap(ctx)
}

func getClientForAddressable(addressable duckv1.Addressable) (*nethttp.Client, error) {
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

	clientKey := addressable.URL.String()

	clientEntry, ok := clients.clients[clientKey]
	var client *nethttp.Client
	if !ok {
		newClient, err := createNewClient(addressable)
		if err != nil {
			return nil, fmt.Errorf("failed to create new client for addressable: %w", err)
		}

		clients.clients[clientKey] = &clientsMapEntry{client: newClient, lastAccessed: time.Now()}

		client = newClient
	} else {
		client = clientEntry.client
		clientEntry.lastAccessed = time.Now()
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
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()

	clientKey := addressable.URL.String()

	client, err := createNewClient(addressable)
	if err != nil {
		fmt.Printf("failed to create new client: %v", err)
		return
	}
	clients.clients[clientKey] = &clientsMapEntry{client: client, lastAccessed: time.Now()}
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
	configureConnectionArgsOldClient(ca) //also configure the connection args of the old client

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
			clientEntry.client.CloseIdleConnections()
		}

		// Resetting clients
		clients.clients = make(map[string]*clientsMapEntry)
	}

	clients.connectionArgs = ca
}

// SetClientTTL sets the ttl before cached clients expire
// This does not update existing clients, it only affects new ones
func SetClientTTL(ttl time.Duration) {
	clients.clientsMu.Lock()
	defer clients.clientsMu.Unlock()
	clients.ttl = ttl
}

// SetClientRecheckInterval sets the interval before the clients map is re-checked for expired entries.
// forceRestart will force the loop to restart with the new interval, cancelling the current iteration.
func SetClientRecheckInterval(recheckInterval time.Duration, forceRestart bool) {
	clients.timerMu.Lock()
	clients.recheckInterval = recheckInterval
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
		t := time.NewTimer(clients.recheckInterval)
		clients.timerMu.Unlock()
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			clients.clientsMu.Lock()
			for k, cme := range clients.clients {
				if time.Now().Sub(cme.lastAccessed) > clients.ttl {
					delete(clients.clients, k)
				}
			}
			clients.clientsMu.Unlock()
		}
	}
}
