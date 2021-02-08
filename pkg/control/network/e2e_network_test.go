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
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/control"
	"knative.dev/eventing/pkg/control/certificates"
)

type mockMessage string

func (m mockMessage) MarshalBinary() (data []byte, err error) {
	return []byte(m), nil
}

func TestTLSConf(t *testing.T) {
	serverTLSConf, clientTLSDialer := testTLSConf(t, context.TODO())
	require.NotNil(t, serverTLSConf)
	require.NotNil(t, clientTLSDialer)
}

func TestStartClientAndServer(t *testing.T) {
	_, _, _, _ = mustSetupWithTLS(t)
}

func TestServerToClient(t *testing.T) {
	_, server, _, client := mustSetupWithTLS(t)
	runSendReceiveTest(t, server, client)
}

func TestClientToServer(t *testing.T) {
	_, server, _, client := mustSetupWithTLS(t)
	runSendReceiveTest(t, client, server)
}

func TestNoopMessageHandlerAcks(t *testing.T) {
	_, server, _, _ := mustSetupWithTLS(t)
	require.NoError(t, server.SendAndWaitForAck(10, mockMessage("Funky!")))
}

func TestInsecureServerToClient(t *testing.T) {
	_, server, _, client := mustSetupInsecure(t)
	runSendReceiveTest(t, server, client)
}

func TestInsecureClientToServer(t *testing.T) {
	_, server, _, client := mustSetupInsecure(t)
	runSendReceiveTest(t, client, server)
}

func TestServerToClientAndBack(t *testing.T) {
	_, server, _, client := mustSetupWithTLS(t)

	wg := sync.WaitGroup{}
	wg.Add(6)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(2), message.Headers().OpCode())
		require.Equal(t, "Funky2!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))
	client.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, server.SendAndWaitForAck(1, mockMessage("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, mockMessage("Funky2!")))
	require.NoError(t, server.SendAndWaitForAck(1, mockMessage("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, mockMessage("Funky2!")))
	require.NoError(t, server.SendAndWaitForAck(1, mockMessage("Funky!")))
	require.NoError(t, client.SendAndWaitForAck(2, mockMessage("Funky2!")))

	wg.Wait()
}

func TestClientToServerWithClientStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := testTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn)
	t.Cleanup(serverCancelFn)

	server, closedServerSignal, err := StartControlServer(serverCtx, serverTLSConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-closedServerSignal
	})

	client, err := StartControlClient(clientCtx, clientTLSDialer, "localhost")
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(2)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	// Send a message, close the client, restart it and send another message

	require.NoError(t, client.SendAndWaitForAck(1, mockMessage("Funky!")))

	clientCancelFn()

	time.Sleep(1 * time.Second)

	clientCtx2, clientCancelFn2 := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn2)
	client2, err := StartControlClient(clientCtx2, clientTLSDialer, "localhost")
	require.NoError(t, err)

	require.NoError(t, client2.SendAndWaitForAck(1, mockMessage("Funky!")))

	wg.Wait()
}

func TestClientToServerWithServerStop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := testTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)
	t.Cleanup(clientCancelFn)
	t.Cleanup(serverCancelFn)

	server, closedServerSignal, err := StartControlServer(serverCtx, serverTLSConf)
	require.NoError(t, err)

	client, err := StartControlClient(clientCtx, clientTLSDialer, "localhost")
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	// Send a message, close the server, restart it and send another message

	require.NoError(t, client.SendAndWaitForAck(1, mockMessage("Funky!")))

	serverCancelFn()

	<-closedServerSignal

	serverCtx2, serverCancelFn2 := context.WithCancel(ctx)
	server2, closedServerSignal, err := StartControlServer(serverCtx2, serverTLSConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn2()
		<-closedServerSignal
	})

	server2.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, client.SendAndWaitForAck(1, mockMessage("Funky!")))

	wg.Wait()
}

func TestManyMessages(t *testing.T) {
	ctx, server, _, client := mustSetupWithTLS(t)

	var wg sync.WaitGroup
	wg.Add(1000 * 2)

	processed := atomic.NewInt32(0)

	server.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(2), message.Headers().OpCode())
		require.Equal(t, "Funky2!", string(message.Payload()))
		message.Ack()
		wg.Done()
		processed.Inc()
	}))
	client.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
		processed.Inc()
	}))

	for i := 0; i < 1000; i++ {
		if i%2 == 0 {
			go func() {
				require.NoError(t, server.SendAndWaitForAck(1, mockMessage("Funky!")))
				wg.Done()
			}()
		} else {
			go func() {
				require.NoError(t, client.SendAndWaitForAck(2, mockMessage("Funky2!")))
				wg.Done()
			}()
		}
	}

	wg.Wait()

	logging.FromContext(ctx).Infof("Processed: %d", processed.Load())
}

func mustGenerateTLSServerConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) func() (*tls.Config, error) {
	return func() (*tls.Config, error) {
		dataPlaneKeyPair, err := certificates.CreateDataPlaneCert(ctx, caKey, caCertificate, 24*time.Hour)
		require.NoError(t, err)

		dataPlaneCert, err := tls.X509KeyPair(dataPlaneKeyPair.CertBytes(), dataPlaneKeyPair.PrivateKeyBytes())
		require.NoError(t, err)

		certPool := x509.NewCertPool()
		certPool.AddCert(caCertificate)
		return &tls.Config{
			Certificates: []tls.Certificate{dataPlaneCert},
			ClientCAs:    certPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ServerName:   caCertificate.DNSNames[0],
		}, nil
	}
}

func mustGenerateTLSClientConf(t *testing.T, ctx context.Context, caKey *rsa.PrivateKey, caCertificate *x509.Certificate) *tls.Config {
	controlPlaneKeyPair, err := certificates.CreateControlPlaneCert(ctx, caKey, caCertificate, 24*time.Hour)
	require.NoError(t, err)

	controlPlaneCert, err := tls.X509KeyPair(controlPlaneKeyPair.CertBytes(), controlPlaneKeyPair.PrivateKeyBytes())
	require.NoError(t, err)

	certPool := x509.NewCertPool()
	certPool.AddCert(caCertificate)
	return &tls.Config{
		Certificates: []tls.Certificate{controlPlaneCert},
		RootCAs:      certPool,
		ServerName:   certificates.FakeDnsName,
	}
}

func testTLSConf(t *testing.T, ctx context.Context) (func() (*tls.Config, error), *tls.Dialer) {
	caKP, err := certificates.CreateCACerts(ctx, 24*time.Hour)
	require.NoError(t, err)
	caCert, caPrivateKey, err := caKP.Parse()
	require.NoError(t, err)

	serverTLSConf := mustGenerateTLSServerConf(t, ctx, caPrivateKey, caCert)
	clientTLSConf := mustGenerateTLSClientConf(t, ctx, caPrivateKey, caCert)

	tlsDialer := &tls.Dialer{
		NetDialer: &net.Dialer{
			KeepAlive: keepAlive,
			Deadline:  time.Time{},
		},
		Config: clientTLSConf,
	}
	return serverTLSConf, tlsDialer
}

func mustSetupWithTLS(t *testing.T) (serverCtx context.Context, server control.Service, clientCtx context.Context, client control.Service) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())
	serverTLSConf, clientTLSDialer := testTLSConf(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	server, closedServerSignal, err := StartControlServer(serverCtx, serverTLSConf)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-closedServerSignal
	})

	client, err = StartControlClient(clientCtx, clientTLSDialer, "127.0.0.1")
	require.NoError(t, err)
	t.Cleanup(clientCancelFn)

	return serverCtx, server, clientCtx, client
}

func mustSetupInsecure(t *testing.T) (serverCtx context.Context, server control.Service, clientCtx context.Context, client control.Service) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	clientCtx, clientCancelFn := context.WithCancel(ctx)
	serverCtx, serverCancelFn := context.WithCancel(ctx)

	server, closedServerSignal, err := StartInsecureControlServer(serverCtx)
	require.NoError(t, err)
	t.Cleanup(func() {
		serverCancelFn()
		<-closedServerSignal
	})

	client, err = StartControlClient(clientCtx, &net.Dialer{
		KeepAlive: keepAlive,
		Deadline:  time.Time{},
	}, "127.0.0.1")
	require.NoError(t, err)
	t.Cleanup(clientCancelFn)

	return serverCtx, server, clientCtx, client
}

func runSendReceiveTest(t *testing.T, sender control.Service, receiver control.Service) {
	wg := sync.WaitGroup{}
	wg.Add(1)

	receiver.MessageHandler(control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
		require.Equal(t, uint8(1), message.Headers().OpCode())
		require.Equal(t, "Funky!", string(message.Payload()))
		message.Ack()
		wg.Done()
	}))

	require.NoError(t, sender.SendAndWaitForAck(1, mockMessage("Funky!")))

	wg.Wait()
}
