/*
 * Copyright 2018 The Knative Authors
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

package channel

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
)

const correlationIDHeaderName = "Knative-Correlation-Id"

type Dispatcher interface {
	// DispatchEvent dispatches an event to a destination over HTTP.
	//
	// The destination and reply are DNS names. For names with a single label,
	// the default namespace is used to expand it into a fully qualified name
	// within the cluster.
	DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply string, defaults DispatchDefaults) error
}

// EventDispatcher is the 'real' Dispatcher used everywhere except unit tests.
var _ Dispatcher = &EventDispatcher{}

var propagation = &b3.HTTPFormat{}

// EventDispatcher dispatches events to a destination over HTTP.
type EventDispatcher struct {
	ceClient         cloudevents.Client
	supportedSchemes sets.String

	logger *zap.Logger
}

// DispatchDefaults provides default parameter values used when dispatching an event.
type DispatchDefaults struct {
	Namespace string
}

// NewEventDispatcher creates a new event dispatcher that can dispatch
// events to HTTP destinations.
func NewEventDispatcher(logger *zap.Logger) *EventDispatcher {
	ceClient, err := kncloudevents.NewDefaultClient()
	if err != nil {
		logger.Fatal("failed to create cloudevents client", zap.Error(err))
	}
	return &EventDispatcher{
		ceClient:         ceClient,
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

// DispatchEvent dispatches an event to a destination over HTTP.
//
// The destination and reply are DNS names. For names with a single label,
// the default namespace is used to expand it into a fully qualified name
// within the cluster.
func (d *EventDispatcher) DispatchEvent(ctx context.Context, event cloudevents.Event, destination, reply string, defaults DispatchDefaults) error {
	var err error
	// Default to replying with the original event. If there is a destination, then replace it
	// with the response from the call to the destination instead.
	response := &event
	if destination != "" {
		destinationURL := d.resolveURL(destination, defaults.Namespace)
		ctx, response, err = d.executeRequest(ctx, destinationURL, event)
		if err != nil {
			return fmt.Errorf("unable to complete request %v", err)
		}
	}

	if reply != "" && response != nil {
		replyURL := d.resolveURL(reply, defaults.Namespace)
		_, _, err = d.executeRequest(ctx, replyURL, *response)
		if err != nil {
			return fmt.Errorf("failed to forward reply %v", err)
		}
	}
	return nil
}

func (d *EventDispatcher) executeRequest(ctx context.Context, url *url.URL, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	d.logger.Info("Dispatching event", zap.String("url", url.String()))

	tctx := addOutGoingTracing(ctx, url)
	sctx := utils.ContextFrom(tctx, url)
	rctx, reply, err := d.ceClient.Send(sctx, event)
	if err != nil {
		return rctx, nil, err
	}
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	if isFailure(rtctx.StatusCode) {
		// reject non-successful responses
		return rctx, nil, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", rtctx.StatusCode)
	}
	headers := utils.PassThroughHeadersFromHeaders(rtctx.Header)
	if correlationID, ok := tctx.Header[correlationIDHeaderName]; ok {
		headers[correlationIDHeaderName] = correlationID
	}
	rtctx.Header = http.Header(headers)
	rctx = cehttp.WithTransportContext(rctx, rtctx)
	return rctx, reply, nil
}

func addOutGoingTracing(ctx context.Context, url *url.URL) cloudevents.HTTPTransportContext {
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	// Creating a dummy request to leverage propagation.SpanContextFromRequest method
	req := &http.Request{
		Header: tctx.Header,
	}
	// Attach the Span context that is currently saved in the request's headers.
	if sc, ok := propagation.SpanContextFromRequest(req); ok {
		newCtx, _ := trace.StartSpanWithRemoteParent(ctx, url.Path, sc)
		return cloudevents.HTTPTransportContextFrom(newCtx)
	}
	return tctx
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < http.StatusOK /* 200 */ ||
		statusCode >= http.StatusMultipleChoices /* 300 */
}

func (d *EventDispatcher) resolveURL(destination string, defaultNamespace string) *url.URL {
	if url, err := url.Parse(destination); err == nil && d.supportedSchemes.Has(url.Scheme) {
		// Already a URL with a known scheme.
		return url
	}
	if strings.Index(destination, ".") == -1 {
		destination = fmt.Sprintf("%s.%s.svc.%s", destination, defaultNamespace, utils.GetClusterDomainName())
	}
	return &url.URL{
		Scheme: "http",
		Host:   destination,
		Path:   "/",
	}
}
