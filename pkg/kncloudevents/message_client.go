/*
 * Copyright 2020 The Knative Authors
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
	"context"
	"fmt"
	"net"
	nethttp "net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"knative.dev/pkg/tracing"
)

const (
	DefaultShutdownTimeout = time.Minute * 1
)

type HttpMessageSender struct {
	Client *nethttp.Client
	Target string
}

func NewHttpMessageSender(connectionArgs *ConnectionArgs, target string) (*HttpMessageSender, error) {
	// Add connection options to the default transport.
	var base = nethttp.DefaultTransport.(*nethttp.Transport).Clone()
	connectionArgs.ConfigureTransport(base)
	// Add output tracing.
	client := &nethttp.Client{
		Transport: &ochttp.Transport{
			Base:        base,
			Propagation: &b3.HTTPFormat{},
		},
	}

	return &HttpMessageSender{Client: client, Target: target}, nil
}

func (s *HttpMessageSender) NewCloudEventRequest(ctx context.Context) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", s.Target, nil)
}

func (s *HttpMessageSender) NewCloudEventRequestWithTarget(ctx context.Context, target string) (*nethttp.Request, error) {
	return nethttp.NewRequestWithContext(ctx, "POST", target, nil)
}

func (s *HttpMessageSender) Send(req *nethttp.Request) (*nethttp.Response, error) {
	return s.Client.Do(req)
}

type HttpMessageReceiver struct {
	port int

	handler  *nethttp.ServeMux
	server   *nethttp.Server
	addr     net.Addr
	listener net.Listener
}

func NewHttpMessageReceiver(port int) *HttpMessageReceiver {
	return &HttpMessageReceiver{
		port: port,
	}
}

// Blocking
func (recv *HttpMessageReceiver) StartListen(ctx context.Context, handler nethttp.Handler) error {
	var err error
	if recv.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", recv.port)); err != nil {
		return err
	}

	recv.handler = nethttp.NewServeMux()

	recv.server = &nethttp.Server{
		Addr:    recv.listener.Addr().String(),
		Handler: tracing.HTTPSpanMiddleware(handler),
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- recv.server.Serve(recv.listener)
	}()

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
		defer cancel()
		err := recv.server.Shutdown(ctx)
		<-errChan // Wait for server goroutine to exit
		return err
	case err := <-errChan:
		return err
	}
}
