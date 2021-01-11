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
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"
	"knative.dev/pkg/network/handlers"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

const (
	DefaultShutdownTimeout = time.Minute * 1
)

type HTTPMessageReceiver struct {
	port int

	server   *http.Server
	listener net.Listener

	checker http.HandlerFunc
}

// HTTPMessageReceiverOption enables further configuration of a HTTPMessageReceiver.
type HTTPMessageReceiverOption func(*HTTPMessageReceiver)

func NewHTTPMessageReceiver(port int, o ...HTTPMessageReceiverOption) *HTTPMessageReceiver {
	h := &HTTPMessageReceiver{
		port: port,
	}
	for _, opt := range o {
		opt(h)
	}
	return h
}

// WithChecker takes a handler func which will run as an additional health check in Drainer.
// kncloudevents HTTPMessageReceiver uses Drainer to perform health check.
// By default, Drainer directly writes StatusOK to kubelet probe if the Pod is not draining.
// Users can configure customized liveness and readiness check logic by defining checker here.
func WithChecker(checker http.HandlerFunc) HTTPMessageReceiverOption {
	return func(h *HTTPMessageReceiver) {
		h.checker = checker
	}
}

// Blocking
func (recv *HTTPMessageReceiver) StartListen(ctx context.Context, handler http.Handler) error {
	var err error
	if recv.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", recv.port)); err != nil {
		return err
	}

	drainer := &handlers.Drainer{
		Inner:       CreateHandler(handler),
		HealthCheck: recv.checker,
	}
	recv.server = &http.Server{
		Addr:    recv.listener.Addr().String(),
		Handler: drainer,
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- recv.server.Serve(recv.listener)
	}()

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		// As we start to shutdown, disable keep-alives to avoid clients hanging onto connections.
		recv.server.SetKeepAlivesEnabled(false)
		drainer.Drain()
		ctx, cancel := context.WithTimeout(context.Background(), getShutdownTimeout(ctx))
		defer cancel()
		err := recv.server.Shutdown(ctx)
		<-errChan // Wait for server goroutine to exit
		return err
	case err := <-errChan:
		return err
	}
}

type shutdownTimeoutKey struct{}

func getShutdownTimeout(ctx context.Context) time.Duration {
	v := ctx.Value(shutdownTimeoutKey{})
	if v == nil {
		return DefaultShutdownTimeout
	}
	return v.(time.Duration)
}

func WithShutdownTimeout(ctx context.Context, timeout time.Duration) context.Context {
	return context.WithValue(ctx, shutdownTimeoutKey{}, timeout)
}

func CreateHandler(handler http.Handler) http.Handler {
	return &ochttp.Handler{
		Propagation: tracecontextb3.TraceContextEgress,
		Handler:     handler,
	}
}
