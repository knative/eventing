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
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"knative.dev/pkg/network"
	"knative.dev/pkg/network/handlers"
	"knative.dev/pkg/observability/tracing"
)

const (
	DefaultShutdownTimeout = time.Minute * 1
	TracerName             = "knative.dev/eventing/pkg/kncloudevents"
)

type HTTPEventReceiver struct {
	desiredPort int
	port        int

	server   *http.Server
	listener net.Listener

	checker          http.HandlerFunc
	drainQuietPeriod time.Duration

	// Used to signal when receiver is listening
	Ready chan interface{}
}

// HTTPEventReceiverOption enables further configuration of a HTTPEventReceiver.
type HTTPEventReceiverOption func(*HTTPEventReceiver)

// NewHTTPEventReceiver creates a new HTTPEventReceiver. When desiredPort is set to 0, a free port will be chosen.
// The final port of the running server will be stored in receiver.Port.
func NewHTTPEventReceiver(desiredPort int, o ...HTTPEventReceiverOption) *HTTPEventReceiver {
	h := &HTTPEventReceiver{
		desiredPort: desiredPort,
		Ready:       make(chan interface{}),
	}

	for _, opt := range o {
		opt(h)
	}
	return h
}

// WithChecker takes a handler func which will run as an additional health check in Drainer.
// kncloudevents HTTPEventReceiver uses Drainer to perform health check.
// By default, Drainer directly writes StatusOK to kubelet probe if the Pod is not draining.
// Users can configure customized liveness and readiness check logic by defining checker here.
func WithChecker(checker http.HandlerFunc) HTTPEventReceiverOption {
	return func(h *HTTPEventReceiver) {
		h.checker = checker
	}
}

// WithDrainQuietPeriod configures the QuietPeriod for the Drainer.
func WithDrainQuietPeriod(duration time.Duration) HTTPEventReceiverOption {
	return func(h *HTTPEventReceiver) {
		h.drainQuietPeriod = duration
	}
}

// WithTLSConfig configures the TLS config for the receiver.
func WithTLSConfig(cfg *tls.Config) HTTPEventReceiverOption {
	return func(h *HTTPEventReceiver) {
		if h.server == nil {
			h.server = newServer()
		}

		h.server.TLSConfig = cfg
	}
}

// WithWriteTimeout sets the HTTP server's WriteTimeout. It covers the time between end of reading
// Request Header to end of writing response.
func WithWriteTimeout(duration time.Duration) HTTPEventReceiverOption {
	return func(h *HTTPEventReceiver) {
		if h.server == nil {
			h.server = newServer()
		}

		h.server.WriteTimeout = duration
	}
}

// WithReadTimeout sets the HTTP server's ReadTimeout. It covers the duration from reading the entire request
// (Headers + Body)
func WithReadTimeout(duration time.Duration) HTTPEventReceiverOption {
	return func(h *HTTPEventReceiver) {
		if h.server == nil {
			h.server = newServer()
		}

		h.server.ReadTimeout = duration
	}
}

func (recv *HTTPEventReceiver) GetAddr() string {
	if recv.server != nil {
		return recv.server.Addr
	}

	return ""
}

// GetPort returns the final assigned port of the server.
// This is blocking, as we need to wait until the server is running.
func (recv *HTTPEventReceiver) GetPort() int {
	// wait until server is ready, as only then the port is assigned
	<-recv.Ready

	return recv.port
}

// Blocking
func (recv *HTTPEventReceiver) StartListen(ctx context.Context, handler http.Handler, otelOpts ...otelhttp.Option) error {
	var err error
	if recv.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", recv.desiredPort)); err != nil {
		return err
	}

	drainer := &handlers.Drainer{
		Inner:       CreateHandler(handler, otelOpts...),
		HealthCheck: recv.checker,
		QuietPeriod: recv.drainQuietPeriod,
	}
	if recv.server == nil {
		recv.server = newServer()
	}
	recv.server.Addr = recv.listener.Addr().String()
	recv.server.Handler = drainer

	_, portStr, err := net.SplitHostPort(recv.server.Addr)
	if err != nil {
		return fmt.Errorf("could not get port of server: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("could not convert port to int: %w", err)
	}

	recv.port = port

	errChan := make(chan error, 1)
	go func() {
		close(recv.Ready)
		if recv.server.TLSConfig == nil {
			errChan <- recv.server.Serve(recv.listener)
		} else {
			errChan <- recv.server.ServeTLS(recv.listener, "", "")
		}
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

func CreateHandler(handler http.Handler, otelOpts ...otelhttp.Option) http.Handler {
	opts := append([]otelhttp.Option{
		otelhttp.WithPropagators(tracing.DefaultTextMapPropagator()),
		otelhttp.WithFilter(func(r *http.Request) bool {
			// Don't trace kubelet probes
			return !network.IsKubeletProbe(r)
		}),
	}, otelOpts...)
	return otelhttp.NewHandler(
		handler,
		"kncloudevents.receive",
		opts...,
	)
}

func newServer() *http.Server {
	return &http.Server{
		ReadTimeout: 10 * time.Second,
	}
}
