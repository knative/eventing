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
	"net/url"
	"sync/atomic"
	"time"

	cloudevents "github.com/cloudevents/sdk-go"
	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"go.uber.org/zap"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/trigger/path"
	"knative.dev/pkg/tracing"
)

const (
	writeTimeout = 1 * time.Minute

	passFilter FilterResult = "pass"
	failFilter FilterResult = "fail"
	noFilter   FilterResult = "no_filter"
)

// Handler parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Handler struct {
	logger        *zap.Logger
	triggerLister eventinglisters.TriggerNamespaceLister
	ceClient      cloudevents.Client
	reporter      StatsReporter
	isReady       *atomic.Value
}

// FilterResult has the result of the filtering operation.
type FilterResult string

// NewHandler creates a new Handler and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned Handler.
func NewHandler(logger *zap.Logger, triggerLister eventinglisters.TriggerNamespaceLister, reporter StatsReporter) (*Handler, error) {
	httpTransport, err := cloudevents.NewHTTPTransport(cloudevents.WithBinaryEncoding(), cehttp.WithMiddleware(tracing.HTTPSpanMiddleware))
	if err != nil {
		return nil, err
	}

	ceClient, err := cloudevents.NewClient(httpTransport, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
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
	httpTransport.Handler.HandleFunc("/readyz", r.readyZ)

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
	originalV3 := event.Context.AsV03()
	ttl, ttlKey := broker.GetTTL(event.Context)
	if ttl == nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		r.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// This doesn't return an error because normally this function is called by a Channel, which
		// will retry all non-2XX responses. If we return an error from this function, then the
		// framework returns a 500 to the caller, so the Channel would send this repeatedly.
		return nil
	}
	delete(originalV3.Extensions, ttlKey)
	event.Context = originalV3

	r.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	responseEvent, err := r.sendEvent(ctx, tctx, triggerRef, &event)
	if err != nil {
		r.logger.Error("Error sending the event", zap.Error(err))
		return err
	}

	resp.Status = http.StatusAccepted
	if responseEvent == nil {
		return nil
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
	responseEvent.Context, err = broker.SetTTL(responseEvent.Context, ttl)
	if err != nil {
		return err
	}
	resp.Event = responseEvent
	resp.Context = &cloudevents.HTTPTransportResponseContext{
		Header: broker.ExtractPassThroughHeaders(tctx),
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
		ns:          t.Namespace,
		trigger:     t.Name,
		broker:      t.Spec.Broker,
		eventType:   triggerFilterAttribute(t.Spec.Filter, "type"),
		eventSource: triggerFilterAttribute(t.Spec.Filter, "source"),
	}

	subscriberURIString := t.Status.SubscriberURI
	if subscriberURIString == "" {
		err = errors.New("unable to read subscriberURI")
		// Record the event count.
		r.reporter.ReportEventCount(reportArgs, err)
		return nil, err
	}
	// We could just send the request to this URI regardless, but let's just check to see if it well
	// formed first, that way we can generate better error message if it isn't.
	subscriberURI, err := url.Parse(subscriberURIString)
	if err != nil {
		r.logger.Error("Unable to parse subscriberURI", zap.Error(err), zap.String("subscriberURIString", subscriberURIString))
		// Record the event count.
		r.reporter.ReportEventCount(reportArgs, err)
		return nil, err
	}

	// Check if the event should be sent, and record filtering time.
	start := time.Now()
	filterResult := r.shouldSendEvent(ctx, &t.Spec, event)
	r.reporter.ReportFilterTime(reportArgs, filterResult, time.Since(start))

	if filterResult == failFilter {
		r.logger.Debug("Event did not pass filter", zap.Any("triggerRef", trigger))
		// Record the event count.
		r.reporter.ReportEventCount(reportArgs, errors.New("event did not pass filter"))
		return nil, nil
	}

	start = time.Now()
	sendingCTX := broker.SendingContext(ctx, tctx, subscriberURI)
	// TODO get HTTP status codes and use those.
	replyEvent, err := r.ceClient.Send(sendingCTX, *event)
	// Record the dispatch time.
	r.reporter.ReportDispatchTime(reportArgs, err, time.Since(start))
	// Record the event count.
	r.reporter.ReportEventCount(reportArgs, err)
	// Record the event latency. This might be off if the receiver and the filter pods are running in
	// different nodes with different clocks.
	var arrivalTime time.Time
	if extErr := event.ExtensionAs(broker.EventArrivalTime, &arrivalTime); extErr == nil {
		r.reporter.ReportEventDeliveryTime(reportArgs, err, time.Since(arrivalTime))
	}
	return replyEvent, err
}

func (r *Handler) getTrigger(ctx context.Context, ref path.NamespacedNameUID) (*eventingv1alpha1.Trigger, error) {
	t, err := r.triggerLister.Get(ref.Name)
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

	result := r.filterEventByAttributes(ctx, attrs, event)
	filterResult := failFilter
	if result {
		filterResult = passFilter
	}
	return filterResult
}

func (r *Handler) filterEventByAttributes(ctx context.Context, attrs map[string]string, event *cloudevents.Event) bool {
	// Set standard context attributes. The attributes available may not be
	// exactly the same as the attributes defined in the current version of the
	// CloudEvents spec.
	ce := map[string]interface{}{
		"specversion":         event.SpecVersion(),
		"type":                event.Type(),
		"source":              event.Source(),
		"subject":             event.Subject(),
		"id":                  event.ID(),
		"time":                event.Time().String(),
		"schemaurl":           event.SchemaURL(),
		"datacontenttype":     event.DataContentType(),
		"datamediatype":       event.DataMediaType(),
		"datacontentencoding": event.DataContentEncoding(),
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
			return false
		}
		// If the attribute is not set to any and is different than the one from the event, return false.
		if v != eventingv1alpha1.TriggerAnyFilter && v != value {
			logging.FromContext(ctx).Debug("Attribute had non-matching value", zap.String("attribute", k), zap.String("filter", v), zap.Any("received", value))
			return false
		}
	}
	return true
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
