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
	"fmt"
	"go.opencensus.io/trace"
	"knative.dev/pkg/logging"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/kncloudevents"
	broker "knative.dev/eventing/pkg/mtbroker"
	"knative.dev/eventing/pkg/reconciler/trigger/path"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const (
	passFilter FilterResult = "pass"
	failFilter FilterResult = "fail"
	noFilter   FilterResult = "no_filter"

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
	// receiver receives incoming HTTP requests
	receiver *kncloudevents.HttpMessageReceiver
	// sender sends requests to downstream services
	sender broker.Sender
	// reporter reports stats of status code and dispatch time
	reporter StatsReporter

	triggerLister eventinglisters.TriggerLister
	logger        *zap.Logger
}

// FilterResult has the result of the filtering operation.
type FilterResult string

// NewHandler creates a new Handler and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned Handler.
func NewHandler(logger *zap.Logger, triggerLister eventinglisters.TriggerLister, reporter StatsReporter, port int) (*Handler, error) {

	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}

	sender, err := kncloudevents.NewHttpMessageSender(&connectionArgs, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create message sender: %w", err)
	}

	return &Handler{
		receiver:      kncloudevents.NewHttpMessageReceiver(port),
		sender:        sender,
		reporter:      reporter,
		triggerLister: triggerLister,
		logger:        logger,
	}, nil
}

// Start begins to receive messages for the handler.
//
// HTTP POST requests to the root path (/) are accepted.
//
// This method will block until ctx is done.
func (h *Handler) Start(ctx context.Context) error {
	return h.receiver.StartListen(ctx, broker.WithLivenessCheck(broker.WithReadinessCheck(h)))
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {

	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	triggerRef, err := path.Parse(request.RequestURI)
	if err != nil {
		h.logger.Info("Unable to parse path as trigger", zap.Error(err), zap.String("path", request.RequestURI))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := binding.ToEvent(request.Context(), cehttp.NewMessageFromHttpRequest(request))
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := request.Context()
	ctx, span := trace.StartSpan(ctx, tracing.TriggerMessagingDestination(triggerRef.NamespacedName))
	defer span.End()

	if span.IsRecordingEvents() {
		span.AddAttributes(
			tracing.MessagingSystemAttribute,
			tracing.MessagingProtocolHTTP,
			tracing.TriggerMessagingDestinationAttribute(triggerRef.NamespacedName),
			tracing.MessagingMessageIDAttribute(event.ID()),
		)
	}

	// Remove the TTL attribute that is used by the Broker.
	ttl, err := broker.GetTTL(event.Context)
	if err != nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		h.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// Return a BadRequest error, so the upstream can decide how to handle it, e.g. sending
		// the message to a DLQ.
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := broker.DeleteTTLv2(event.Context); err != nil {
		h.logger.Warn("Failed to delete TTL.", zap.Error(err))
	}

	h.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	t, err := h.getTrigger(ctx, triggerRef)
	if err != nil {
		h.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", triggerRef))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	reportArgs := &ReportArgs{
		ns:         t.Namespace,
		trigger:    t.Name,
		broker:     t.Spec.Broker,
		filterType: triggerFilterAttribute(t.Spec.Filter, "type"),
	}

	subscriberURI := t.Status.SubscriberURI
	if subscriberURI == nil {
		// Record the event count.
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusNotFound)
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte("unable to get subscriber URI"))
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusBadRequest)
		return
	}

	// Check if the event should be sent.
	filterResult := h.shouldSendEvent(ctx, &t.Spec, event)

	if filterResult == failFilter {
		h.logger.Debug("Event did not pass filter", zap.Any("triggerRef", triggerRef))
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// Record the event processing time. This might be off if the receiver and the filter pods are running in
	// different nodes with different clocks.
	var arrivalTimeStr string
	if extErr := event.ExtensionAs(broker.EventArrivalTime, &arrivalTimeStr); extErr == nil {
		arrivalTime, err := time.Parse(time.RFC3339, arrivalTimeStr)
		if err == nil {
			_ = h.reporter.ReportEventProcessingTime(reportArgs, time.Since(arrivalTime))
		}
	}

	// Send the event to the subscriber
	target := subscriberURI.String()
	req, err := h.sender.NewCloudEventRequestWithTarget(ctx, target)
	if err != nil {
		h.logger.Info("failed to create the request", zap.String("target", target))
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	req.Header = utils.PassThroughHeaders(request.Header)
	err = cehttp.WriteRequest(ctx, binding.ToMessage(event), req)
	if err != nil {
		h.logger.Info("failed to write request")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	start := time.Now()
	resp, err := h.sender.Send(req)
	dispatchTime := time.Since(start)
	_ = h.reporter.ReportEventDispatchTime(reportArgs, http.StatusInternalServerError, dispatchTime)
	if err != nil {
		h.logger.Error("failed to dispatch message", zap.Error(err))
		writer.WriteHeader(http.StatusServiceUnavailable)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusServiceUnavailable)
		return
	}

	response := cehttp.NewMessageFromHttpResponse(resp)
	event, err = binding.ToEvent(ctx, response)
	if err != nil {
		h.logger.Error("failed to convert message to event", zap.Error(err))
		// No event in the response we're golden.
		writer.WriteHeader(resp.StatusCode)
		_ = h.reporter.ReportEventCount(reportArgs, resp.StatusCode)
		return
	}
	if event == nil {
		// No event in the response we're golden.
		writer.WriteHeader(http.StatusAccepted)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusAccepted)
		return
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
	if err := broker.SetTTLv2(event.Context, ttl); err != nil {
		h.logger.Error("failed to reset TTL", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
		return
	}

	eventResponse := binding.ToMessage(event)
	if err := cehttp.WriteResponseWriter(ctx, eventResponse, resp.StatusCode, writer); err != nil {
		h.logger.Error("failed to write response event", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
		return
	}

	_ = h.reporter.ReportEventCount(reportArgs, resp.StatusCode)
}

func (r *Handler) getTrigger(ref path.NamespacedNameUID) (*eventingv1beta1.Trigger, error) {
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
func (r *Handler) shouldSendEvent(ctx context.Context, ts *eventingv1beta1.TriggerSpec, event *cloudevents.Event) FilterResult {
	// No filter specified, default to passing everything.
	if ts.Filter == nil || len(ts.Filter.Attributes) == 0 {
		return noFilter
	}
	return filterEventByAttributes(ctx, map[string]string(ts.Filter.Attributes), event)
}

func filterEventByAttributes(ctx context.Context, attrs map[string]string, event *cloudevents.Event) FilterResult {
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
	for k, v := range ext {
		ce[k] = v
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
		if v != eventingv1beta1.TriggerAnyFilter && v != value {
			logging.FromContext(ctx).Debug("Attribute had non-matching value", zap.String("attribute", k), zap.String("filter", v), zap.Any("received", value))
			return failFilter
		}
	}
	return passFilter
}

// triggerFilterAttribute returns the filter attribute value for a given `attributeName`. If it doesn't not exist,
// returns the any value filter.
func triggerFilterAttribute(filter *eventingv1beta1.TriggerFilter, attributeName string) string {
	attributeValue := eventingv1beta1.TriggerAnyFilter
	if filter != nil {
		attrs := map[string]string(filter.Attributes)
		if v, ok := attrs[attributeName]; ok {
			attributeValue = v
		}
	}
	return attributeValue
}
