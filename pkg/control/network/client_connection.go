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

package network

import (
	"context"
	"fmt"
	"net"
	"time"

	"knative.dev/pkg/logging"

	ctrl "knative.dev/eventing/pkg/control"
	ctrlservice "knative.dev/eventing/pkg/control/service"
)

type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

func StartControlClient(ctx context.Context, dialOptions Dialer, target string) (ctrl.Service, error) {
	target = target + ":9000"
	logging.FromContext(ctx).Infof("Starting control client to %s", target)

	// Let's try the dial
	conn, err := tryDial(ctx, dialOptions, target, clientInitialDialRetry, clientDialRetryInterval)
	if err != nil {
		return nil, fmt.Errorf("cannot perform the initial dial to target %s: %w", target, err)
	}

	tcpConn := newClientTcpConnection(ctx, dialOptions)
	svc := ctrlservice.NewService(ctx, tcpConn)

	tcpConn.startPolling(conn)

	return svc, nil
}

func tryDial(ctx context.Context, dialOptions Dialer, target string, retries int, interval time.Duration) (net.Conn, error) {
	var conn net.Conn
	var err error
	for i := 0; i < retries; i++ {
		conn, err = dialOptions.DialContext(ctx, "tcp", target)
		if err == nil {
			return conn, nil
		}
		logging.FromContext(ctx).Warnf("Error while trying to connect: %v", err)
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(interval):
		}
	}
	return nil, err
}

type clientTcpConnection struct {
	baseTcpConnection

	dialer Dialer
}

func newClientTcpConnection(ctx context.Context, dialer Dialer) *clientTcpConnection {
	c := &clientTcpConnection{
		baseTcpConnection: baseTcpConnection{
			ctx:                    ctx,
			logger:                 logging.FromContext(ctx),
			outboundMessageChannel: make(chan *ctrl.OutboundMessage, 10),
			inboundMessageChannel:  make(chan *ctrl.InboundMessage, 10),
			errors:                 make(chan error, 10),
		},
		dialer: dialer,
	}
	return c
}

func (t *clientTcpConnection) startPolling(initialConn net.Conn) {
	// We have 2 goroutines:
	// * One consumes the connections and it eventually reconnects
	// * One blocks on context done and closes the connection
	go func(initialConn net.Conn) {
		// Consume the connection
		t.consumeConnection(initialConn)

		// Retry until connection closed
		for {
			select {
			case <-t.ctx.Done():
				return
			default:
				t.logger.Warnf("Connection lost, retrying to reconnect %s", initialConn.RemoteAddr().String())

				// Let's try the dial
				conn, err := tryDial(t.ctx, t.dialer, initialConn.RemoteAddr().String(), clientReconnectionRetry, clientDialRetryInterval)
				if err != nil {
					t.logger.Warnf("Cannot re-dial to target %s: %v", initialConn.RemoteAddr().String(), err)
					return
				}

				t.consumeConnection(conn)
			}
		}
	}(initialConn)
	go func() {
		<-t.ctx.Done()
		t.logger.Infof("Closing control client")
		err := t.close()
		t.logger.Infof("Connection closed")
		if err != nil {
			t.logger.Warnf("Error while closing the connection: %s", err)
		}
	}()
}
