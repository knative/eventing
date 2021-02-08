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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"time"

	"knative.dev/pkg/logging"

	ctrl "knative.dev/eventing/pkg/control"
	"knative.dev/eventing/pkg/control/certificates"
	"knative.dev/eventing/pkg/control/service"
)

const (
	baseCertsPath = "/etc/control-secret"

	publicCertPath = baseCertsPath + "/" + certificates.SecretCertKey
	pkPath         = baseCertsPath + "/" + certificates.SecretPKKey
	caCertPath     = baseCertsPath + "/" + certificates.SecretCaCertKey
)

var listenConfig = net.ListenConfig{
	KeepAlive: 30 * time.Second,
}

func LoadServerTLSConfigFromFile() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(publicCertPath, pkPath)
	if err != nil {
		return nil, err
	}

	caCert, err := ioutil.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(caCert)
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ServerName:   certificates.FakeDnsName,
	}

	return conf, nil
}

func StartInsecureControlServer(ctx context.Context) (ctrl.Service, <-chan struct{}, error) {
	return StartControlServer(ctx, nil)
}

func StartControlServer(ctx context.Context, tlsConfigLoader func() (*tls.Config, error)) (ctrl.Service, <-chan struct{}, error) {
	ln, err := listenConfig.Listen(ctx, "tcp", ":9000")
	if err != nil {
		return nil, nil, err
	}

	tcpConn := newServerTcpConnection(ctx, ln, tlsConfigLoader)
	ctrlService := service.NewService(ctx, tcpConn)

	logging.FromContext(ctx).Infof("Started listener: %s", ln.Addr().String())

	closedServerCh := make(chan struct{})

	tcpConn.startAcceptPolling(closedServerCh)

	return ctrlService, closedServerCh, nil
}

type serverTcpConnection struct {
	baseTcpConnection

	listener        net.Listener
	tlsConfigLoader func() (*tls.Config, error)
}

func newServerTcpConnection(ctx context.Context, listener net.Listener, loader func() (*tls.Config, error)) *serverTcpConnection {
	c := &serverTcpConnection{
		baseTcpConnection: baseTcpConnection{
			ctx:                    ctx,
			logger:                 logging.FromContext(ctx),
			outboundMessageChannel: make(chan *ctrl.OutboundMessage, 10),
			inboundMessageChannel:  make(chan *ctrl.InboundMessage, 10),
			errors:                 make(chan error, 10),
		},
		listener:        listener,
		tlsConfigLoader: loader,
	}
	return c
}

func (t *serverTcpConnection) startAcceptPolling(closedServerChannel chan struct{}) {
	// We have 2 goroutines:
	// * One polls the listener to accept new conns
	// * One blocks on context done and closes the listener and the connection
	go func() {
		for {
			conn, err := t.listener.Accept()
			if err != nil {
				t.logger.Warnf("Error while accepting the connection, closing the accept loop: %s", err)
				return
			}
			if t.tlsConfigLoader != nil {
				t.logger.Debugf("Upgrading conn %v to tls", conn.RemoteAddr())
				tlsConf, err := t.tlsConfigLoader()
				if err != nil {
					t.logger.Warnf("Cannot load tls configuration: %v", err)
					t.tryPropagateError(t.ctx, err)
					_ = conn.Close()
					continue
				}
				conn = tls.Server(conn, tlsConf)
			}
			t.logger.Debugf("Accepting new control connection from %s", conn.RemoteAddr())
			t.consumeConnection(conn)
			select {
			case <-t.ctx.Done():
				return
			default:
				continue
			}
		}
	}()
	go func() {
		<-t.ctx.Done()
		t.logger.Infof("Closing control server")
		err := t.listener.Close()
		t.logger.Infof("Listener closed")
		if err != nil {
			t.logger.Warnf("Error while closing the listener: %s", err)
		}
		err = t.close()
		if err != nil {
			t.logger.Warnf("Error while closing the tcp connection: %s", err)
		}
		close(closedServerChannel)
	}()
}
