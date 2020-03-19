/*
 * Copyright 2019 The Knative Authors
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

package filter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v1"
	"go.uber.org/zap"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/trigger/path"
	"knative.dev/eventing/pkg/utils"
	pkgtracing "knative.dev/pkg/tracing"
)

const (
	writeTimeout = 15 * time.Minute

	passFilter FilterResult = "pass"
	failFilter FilterResult = "fail"
	noFilter   FilterResult = "no_filter"

	// readyz is the HTTP path that will be used for readiness checks.
	readyz = "/readyz"

	// TODO make these constants configurable (either as env variables, config map, or part of broker spec).
	//  Issue: https://github.com/knative/eventing/issues/1777
	// Constants for the underlying HTTP Client transport. These would enable better connection reuse.
	// Set them on a 10:1 ratio, but this would actually depend on the Triggers' subscribers and the workload itself.
	// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
	// based on what serving is doing. See https://github.com/knative/serving/blob/master/pkg/network/transports.go.
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 100
)

// Handler parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Handler struct {
	logger        *zap.Logger
	triggerLister eventinglisters.TriggerLister
	ceClient      cloudevents.Client
	reporter      StatsReporter
	isReady       *atomic.Value
}

type sendError struct {
	Err    error
	Status int
}

func (e sendError) Error() string {
	return e.Err.Error()
}

func (e sendError) Unwrap() error {
	return e.Err
}

// FilterResult has the result of the filtering operation.
type FilterResult string

// NewHandler creates a new Handler and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned Handler.
func NewHandler(logger *zap.Logger, triggerLister eventinglisters.TriggerLister, reporter StatsReporter, port int) (*Handler, error) {
	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cloudevents.WithMiddleware(pkgtracing.HTTPSpanIgnoringPaths(readyz)), cloudevents.WithPort(port))
	if err != nil {
		return nil, err
	}

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	ceClient, err := kncloudevents.NewDefaultClientGivenHttpTransport(httpTransport, &connectionArgs)
	if err != nil {
		return nil, err
	}

	r := &Handler{
		logger:        logger,
		triggerLister: triggerLister,
		ceClient:      ceClient,
		reporter:      reporter,
		isReady:       &atomic.Value{},
	}
	r.isReady.Store(false)

	httpTransport.Handler = http.NewServeMux()
	httpTransport.Handler.HandleFunc("/healthz", r.healthZ)
	httpTransport.Handler.HandleFunc(readyz, r.readyZ)

	return r, nil
}

func (r *Handler) healthZ(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func (r *Handler) readyZ(writer http.ResponseWriter, _ *http.Request) {
	if r.isReady == nil || !r.isReady.Load().(bool) {
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	writer.WriteHeader(http.StatusOK)
}

// Start begins to receive messages for the handler.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
//
// This method will block until a message is received on the stop channel.
func (r *Handler) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- r.ceClient.StartReceiver(ctx, r.serveHTTP)
	}()

	// We are ready.
	r.isReady.Store(true)

	// Stop either if the receiver stops (sending to errCh) or if stopCh is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		break
	}

	// No longer ready.
	r.isReady.Store(false)

	// stopCh has been closed, we need to gracefully shutdown h.ceClient. cancel() will start its
	// shutdown, if it hasn't finished in a reasonable amount of time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(writeTimeout):
		return errors.New("timeout shutting down ceClient")
	}
}

func (r *Handler) serveHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the path...
	triggerRef, err := path.Parse(tctx.URI)
	if err != nil {
		r.logger.Info("Unable to parse path as a trigger", zap.Error(err), zap.String("path", tctx.URI))
		return errors.New("unable to parse path as a Trigger")
	}

	// Remove the TTL attribute that is used by the Broker.
	ttl, err := broker.GetTTL(event.Context)
	if err != nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		r.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// Return a BadRequest error, so the upstream can decide how to handle it, e.g. sending
		// the message to a DLQ.
		resp.Status = http.StatusBadRequest
		return nil
	}
	if err := broker.DeleteTTL(event.Context); err != nil {
		r.logger.Warn("Failed to delete TTL.", zap.Error(err))
	}

	r.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	responseEvent, err := r.sendEvent(ctx, tctx, triggerRef, &event)
	if err != nil {
		// Propagate any error codes from the invoke back upstram.
		var httpError sendError
		if errors.As(err, &httpError) {
			resp.Status = httpError.Status
		}
		r.logger.Error("Error sending the event", zap.Error(err))
		return err
	}

	resp.Status = http.StatusAccepted
	if responseEvent == nil {
		return nil
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.

	if err := broker.SetTTL(responseEvent.Context, ttl); err != nil {
		return err
	}
	resp.Event = responseEvent
	resp.Context = &cloudevents.HTTPTransportResponseContext{
		Header: utils.PassThroughHeaders(tctx.Header),
	}

	return nil
}

// sendEvent sends an event to a subscriber if the trigger filter passes.
func (r *Handler) sendEvent(ctx context.Context, tctx cloudevents.HTTPTransportContext, trigger path.NamespacedNameUID, event *cloudevents.Event) (*cloudevents.Event, error) {
	t, err := r.getTrigger(ctx, trigger)
	if err != nil {
		r.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", trigger))
		return nil, err
	}

	reportArgs := &ReportArgs{
		ns:         t.Namespace,
		trigger:    t.Name,
		broker:     t.Spec.Broker,
		filterType: triggerFilterAttribute(t.Spec.Filter, "type"),
	}

	subscriberURI := t.Status.SubscriberURI
	if subscriberURI == nil {
		err = errors.New("unable to read subscriberURI")
		// Record the event count.
		r.reporter.ReportEventCount(reportArgs, http.StatusNotFound)
		return nil, err
	}

	// Check if the event should be sent.
	filterResult := r.shouldSendEvent(ctx, &t.Spec, event)

	if filterResult == failFilter {
		r.logger.Debug("Event did not pass filter", zap.Any("triggerRef", trigger))
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		return nil, nil
	}

	// Record the event processing time. This might be off if the receiver and the filter pods are running in
	// different nodes with different clocks.
	var arrivalTimeStr string
	if extErr := event.ExtensionAs(broker.EventArrivalTime, &arrivalTimeStr); extErr == nil {
		arrivalTime, err := time.Parse(time.RFC3339, arrivalTimeStr)
		if err == nil {
			r.reporter.ReportEventProcessingTime(reportArgs, time.Since(arrivalTime))
		}
	}

	sendingCTX := utils.SendingContextFrom(ctx, tctx, subscriberURI.URL())

	start := time.Now()
	rctx, replyEvent, err := r.ceClient.Send(sendingCTX, *event)
	rtctx := cloudevents.HTTPTransportContextFrom(rctx)
	// Record the dispatch time.
	r.reporter.ReportEventDispatchTime(reportArgs, rtctx.StatusCode, time.Since(start))
	// Record the event count.
	r.reporter.ReportEventCount(reportArgs, rtctx.StatusCode)
	// Wrap any errors along with the response status code so that can be propagated upstream.
	if err != nil {
		err = sendError{err, rtctx.StatusCode}
	}
	return replyEvent, err
}

func (r *Handler) getTrigger(ctx context.Context, ref path.NamespacedNameUID) (*eventingv1alpha1.Trigger, error) {
	t, err := r.triggerLister.Triggers(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, err
	}
	if t.UID != ref.UID {
		return nil, fmt.Errorf("trigger had a different UID. From ref '%s'. From Kubernetes '%s'", ref.UID, t.UID)
	}
	return t, nil
}

// shouldSendEvent determines whether event 'event' should be sent based on the triggerSpec 'ts'.
// Currently it supports exact matching on event context attributes and extension attributes.
// If no filter is present, shouldSendEvent returns passFilter.
func (r *Handler) shouldSendEvent(ctx context.Context, ts *eventingv1alpha1.TriggerSpec, event *cloudevents.Event) FilterResult {
	// No filter specified, default to passing everything.
	if ts.Filter == nil || (ts.Filter.DeprecatedSourceAndType == nil && ts.Filter.Attributes == nil) {
		return noFilter
	}

	attrs := map[string]string{}
	// Since the filters cannot distinguish presence, filtering for an empty
	// string is impossible.
	if ts.Filter.DeprecatedSourceAndType != nil {
		attrs["type"] = ts.Filter.DeprecatedSourceAndType.Type
		attrs["source"] = ts.Filter.DeprecatedSourceAndType.Source
	} else if ts.Filter.Attributes != nil {
		attrs = map[string]string(*ts.Filter.Attributes)
	}

	return r.filterEventByAttributes(ctx, attrs, event)
}

func (r *Handler) filterEventByAttributes(ctx context.Context, attrs map[string]string, event *cloudevents.Event) FilterResult {
	// Set standard context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	ce := map[string]interface{}{
		"specversion":     event.SpecVersion(),
		"type":            event.Type(),
		"source":          event.Source(),
		"subject":         event.Subject(),
		"id":              event.ID(),
		"time":            event.Time().String(),
		"schemaurl":       event.DataSchema(),
		"datacontenttype": event.DataContentType(),
		"datamediatype":   event.DataMediaType(),
		// TODO: use data_base64 when SDK supports it.
		"datacontentencoding": event.DeprecatedDataContentEncoding(),
	}
	ext := event.Extensions()
	if ext != nil {
		for k, v := range ext {
			ce[k] = v
		}
	}

	for k, v := range attrs {
		var value interface{}
		value, ok := ce[k]
		// If the attribute does not exist in the event, return false.
		if !ok {
			logging.FromContext(ctx).Debug("Attribute not found", zap.String("attribute", k))
			return failFilter
		}
		// If the attribute is not set to any and is different than the one from the event, return false.
		if v != eventingv1alpha1.TriggerAnyFilter && v != value {
			logging.FromContext(ctx).Debug("Attribute had non-matching value", zap.String("attribute", k), zap.String("filter", v), zap.Any("received", value))
			return failFilter
		}
	}
	return passFilter
}

// triggerFilterAttribute returns the filter attribute value for a given `attributeName`. If it doesn't not exist,
// returns the any value filter.
func triggerFilterAttribute(filter *eventingv1alpha1.TriggerFilter, attributeName string) string {
	attributeValue := eventingv1alpha1.TriggerAnyFilter
	if filter != nil {
		if filter.DeprecatedSourceAndType != nil {
			if attributeName == "type" {
				attributeValue = filter.DeprecatedSourceAndType.Type
			} else if attributeName == "source" {
				attributeValue = filter.DeprecatedSourceAndType.Source
			}
		} else if filter.Attributes != nil {
			attrs := map[string]string(*filter.Attributes)
			if v, ok := attrs[attributeName]; ok {
				attributeValue = v
			}
		}
	}
	return attributeValue
}
