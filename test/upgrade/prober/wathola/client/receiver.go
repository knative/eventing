/*
 * Copyright 2020 The Knative Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package client

import (
	"context"
	nethttp "net/http"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/wavesoftware/go-ensure"
	"go.opencensus.io/plugin/ochttp"
	"knative.dev/eventing/test/upgrade/prober/wathola/config"
	"knative.dev/pkg/tracing/propagation/tracecontextb3"
)

var log = config.Log

// ReceiveEvent represents a function that receive event
type ReceiveEvent func(e cloudevents.Event)

// Receive events and push then to passed fn
func Receive(
	port int,
	canceling chan context.CancelFunc,
	receiveEvent ReceiveEvent,
	middlewares ...cloudeventshttp.Middleware,
) {
	opts := make([]cloudeventshttp.Option, 0)
	opts = append(opts, cloudevents.WithPort(port))
	opts = append(opts, cloudevents.WithRoundTripper(&ochttp.Transport{
		Propagation: tracecontextb3.TraceContextEgress,
	}))
	if config.Instance.Readiness.Enabled {
		readyOpt := cloudevents.WithMiddleware(readinessMiddleware)
		opts = append(opts, readyOpt)
	}
	for _, m := range middlewares {
		opt := cloudevents.WithMiddleware(m)
		opts = append(opts, opt)
	}
	http, err := cloudevents.NewHTTP(opts...)
	if err != nil {
		log.Fatalf("failed to create http transport, %v", err)
	}
	c, err := cloudevents.NewClient(http)
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}
	log.Infof("Listening for events on port %v", port)
	ctx, cancel := context.WithCancel(context.Background())
	cancelFunc := func() {
		log.Infof("Stopping event receiver on port %v", port)
		cancel()
	}
	// https://gobyexample.com/non-blocking-channel-operations
	select {
	case canceling <- cancelFunc:
	default:
	}
	err = c.StartReceiver(ctx, receiveEvent)
	if err != nil {
		log.Fatal(err)
	}
}

func readinessMiddleware(next nethttp.Handler) nethttp.Handler {
	log.Debugf("Using readiness probe: %v", config.Instance.Readiness.URI)
	return &readinessProbe{
		next: next,
	}
}

type readinessProbe struct {
	next nethttp.Handler
}

func (r readinessProbe) ServeHTTP(rw nethttp.ResponseWriter, req *nethttp.Request) {
	if req.RequestURI == config.Instance.Readiness.URI {
		rw.WriteHeader(config.Instance.Readiness.Status)
		_, err := rw.Write([]byte(config.Instance.Readiness.Message))
		ensure.NoError(err)
		log.Debugf("Received ready check. Headers: %v", headersOf(req))
	} else {
		r.next.ServeHTTP(rw, req)
	}
}

func headersOf(req *nethttp.Request) string {
	var b strings.Builder
	ensure.NoError(req.Header.Write(&b))
	headers := b.String()
	return strings.ReplaceAll(headers, "\r\n", "; ")
}
