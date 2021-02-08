/*
Copyright 2021 The Knative Authors

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

package reconciler

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/control"
	"knative.dev/eventing/pkg/control/network"
)

const (
	keepAlive = 30 * time.Second
)

type ControlPlaneConnectionPool struct {
	tlsDialerFactory TLSDialerFactory
	baseDialOptions  *net.Dialer

	serviceWrapperFactories []control.ServiceWrapper

	connsLock sync.Mutex
	conns     map[string]map[string]clientServiceHolder
}

type clientServiceHolder struct {
	service  control.Service
	cancelFn context.CancelFunc
}

func NewInsecureControlPlaneConnectionPool(opts ...ControlPlaneConnectionPoolOption) *ControlPlaneConnectionPool {
	return NewControlPlaneConnectionPool(nil, opts...)
}

func NewControlPlaneConnectionPool(tlsDialerFactory TLSDialerFactory, opts ...ControlPlaneConnectionPoolOption) *ControlPlaneConnectionPool {
	pool := &ControlPlaneConnectionPool{
		tlsDialerFactory: tlsDialerFactory,
		baseDialOptions: &net.Dialer{
			KeepAlive: keepAlive,
			Deadline:  time.Time{},
		},
		conns: make(map[string]map[string]clientServiceHolder),
	}

	for _, fn := range opts {
		fn(pool)
	}

	return pool
}

func (cc *ControlPlaneConnectionPool) GetConnectedHosts(key string) []string {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	var m map[string]clientServiceHolder
	var ok bool
	if m, ok = cc.conns[key]; !ok {
		return nil
	}
	hosts := make([]string, 0, len(m))
	for k := range m {
		hosts = append(hosts, k)
	}
	return hosts
}

func (cc *ControlPlaneConnectionPool) GetServices(key string) map[string]control.Service {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	var m map[string]clientServiceHolder
	var ok bool
	if m, ok = cc.conns[key]; !ok {
		return nil
	}
	svcs := make(map[string]control.Service, len(m))
	for k, h := range m {
		svcs[k] = h.service
	}
	return svcs
}

func (cc *ControlPlaneConnectionPool) ResolveControlInterface(key string, host string) (string, control.Service) {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	if m, ok := cc.conns[key]; !ok {
		return "", nil
	} else if holder, ok := m[host]; !ok {
		return host, holder.service
	}

	return "", nil
}

func (cc *ControlPlaneConnectionPool) RemoveConnection(ctx context.Context, key string, host string) {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	m, ok := cc.conns[key]
	if !ok {
		return
	}
	holder, ok := m[host]
	if !ok {
		return
	}
	holder.cancelFn()
	delete(m, host)
	if len(m) == 0 {
		delete(cc.conns, key)
	}
}

func (cc *ControlPlaneConnectionPool) RemoveAllConnections(ctx context.Context, key string) {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	m, ok := cc.conns[key]
	if !ok {
		return
	}
	for _, holder := range m {
		holder.cancelFn()
	}
	delete(cc.conns, key)
}

func (cc *ControlPlaneConnectionPool) Close(ctx context.Context) {
	cc.connsLock.Lock()
	defer cc.connsLock.Unlock()
	for _, m := range cc.conns {
		for _, holder := range m {
			holder.cancelFn()
		}
	}
	// Let's make sure this object is reusable
	cc.conns = make(map[string]map[string]clientServiceHolder)
}

func (cc *ControlPlaneConnectionPool) ReconcileConnections(ctx context.Context, key string, wantConnections []string, newServiceCb func(string, control.Service), oldServiceCb func(string)) (map[string]control.Service, error) {
	existingConnections := cc.GetConnectedHosts(key)

	newConnections := setDifference(wantConnections, existingConnections)
	oldConnections := setDifference(existingConnections, wantConnections)

	logging.FromContext(ctx).Debugf("New connections: %v", newConnections)
	logging.FromContext(ctx).Debugf("Old connections: %v", oldConnections)

	for _, newConn := range newConnections {
		logging.FromContext(ctx).Debugf("Creating a new control connection: %s", newConn)

		// Dial the service
		_, ctrl, err := cc.DialControlService(ctx, key, newConn)
		if err != nil {
			return nil, fmt.Errorf("cannot connect to the pod: %w", err)
		}

		if newServiceCb != nil {
			newServiceCb(newConn, ctrl)
		}
	}

	for _, oldConn := range oldConnections {
		logging.FromContext(ctx).Debugf("Cleaning up old connection: %s", oldConn)
		if oldServiceCb != nil {
			oldServiceCb(oldConn)
		}
		cc.RemoveConnection(ctx, key, oldConn)
	}

	logging.FromContext(ctx).Debugf("Now connected to: %v", cc.GetConnectedHosts(key))

	return cc.GetServices(key), nil
}

func (cc *ControlPlaneConnectionPool) DialControlService(ctx context.Context, key string, host string) (string, control.Service, error) {
	var dialer network.Dialer
	dialer = cc.baseDialOptions
	// Check if tlsDialerFactory is set up, otherwise connect without tls
	if cc.tlsDialerFactory != nil {
		// Create TLS dialer
		var err error
		dialer, err = cc.tlsDialerFactory.GenerateTLSDialer(cc.baseDialOptions)
		if err != nil {
			return "", nil, err
		}
	}

	// Need to start new conn
	ctx, cancelFn := context.WithCancel(ctx)
	newSvc, err := network.StartControlClient(ctx, dialer, host)
	if err != nil {
		cancelFn()
		return "", nil, err
	}

	// Apply wrappers
	for _, wrap := range cc.serviceWrapperFactories {
		newSvc = wrap(newSvc)
	}

	cc.connsLock.Lock()
	var m map[string]clientServiceHolder
	var ok bool
	if m, ok = cc.conns[key]; !ok {
		m = make(map[string]clientServiceHolder)
		cc.conns[key] = m
	}
	m[host] = clientServiceHolder{
		service:  newSvc,
		cancelFn: cancelFn,
	}
	cc.connsLock.Unlock()

	return host, newSvc, nil
}

func setDifference(a, b []string) (diff []string) {
	m := make(map[string]bool)

	for _, item := range b {
		m[item] = true
	}

	for _, item := range a {
		if _, ok := m[item]; !ok {
			diff = append(diff, item)
		}
	}
	return
}
