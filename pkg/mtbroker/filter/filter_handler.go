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
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	eventingv1beta1 "knative.dev/eventing/pkg/apis/eventing/v1beta1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1beta1"
	"knative.dev/eventing/pkg/health"
	"knative.dev/eventing/pkg/kncloudevents"
	broker "knative.dev/eventing/pkg/mtbroker"
	"knative.dev/eventing/pkg/reconciler/sugar/trigger/path"
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
	sender *kncloudevents.HttpMessageSender
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
	return h.receiver.StartListen(ctx, health.WithLivenessCheck(health.WithReadinessCheck(h)))
}

// 1. validate request
// 2. extract event from request
// 3. get trigger from its trigger reference extracted from the request URI
// 4. filter event
// 5. send event to trigger's subscriber
// 6. write the response
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

	ctx := request.Context()

	message := cehttp.NewMessageFromHttpRequest(request)
	defer message.Finish(nil)

	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		h.logger.Warn("failed to extract event from request", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx, span := trace.StartSpan(ctx, tracing.TriggerMessagingDestination(triggerRef.NamespacedName))
	defer span.End()

	if span.IsRecordingEvents() {
		span.AddAttributes(
			tracing.MessagingSystemAttribute,
			tracing.MessagingProtocolHTTP,
			tracing.TriggerMessagingDestinationAttribute(triggerRef.NamespacedName),
			tracing.MessagingMessageIDAttribute(event.ID()),
		)
		span.AddAttributes(client.EventTraceAttributes(event)...)
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
	if err := broker.DeleteTTL(event.Context); err != nil {
		h.logger.Warn("Failed to delete TTL.", zap.Error(err))
	}

	h.logger.Debug("Received message", zap.Any("triggerRef", triggerRef))

	t, err := h.getTrigger(triggerRef)
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
		writer.WriteHeader(http.StatusBadRequest)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusBadRequest)
		return
	}

	// Check if the event should be sent.
	ctx = logging.WithLogger(ctx, h.logger.Sugar())
	filterResult := h.shouldSendEvent(ctx, &t.Spec, event)

	if filterResult == failFilter {
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		return
	}

	h.reportArrivalTime(event, reportArgs)

	h.send(ctx, writer, request.Header, subscriberURI.String(), reportArgs, event, ttl, span)
}

func (h *Handler) send(ctx context.Context, writer http.ResponseWriter, headers http.Header, target string, reportArgs *ReportArgs, event *cloudevents.Event, ttl int32, span *trace.Span) {

	// send the event to trigger's subscriber
	response, err := h.sendEvent(ctx, headers, target, event, reportArgs, span)
	if err != nil {
		h.logger.Error("failed to send event", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
		return
	}

	// If there is an event in the response write it to the response
	statusCode, err := writeResponse(ctx, writer, response, ttl, span)
	if err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
		// Ok, so writeResponse will return the HttpStatus of the function. That may have
		// succeeded (200), but it may have returned a malformed event, so if the
		// function succeeded, convert this to an StatusBadGateway instead to indicate
		// error. Note that we could just use StatusInternalServerError, but to distinguish
		// between the two failure cases, we use a different code here.
		if statusCode == 200 {
			statusCode = http.StatusBadGateway
		}
	}
	_ = h.reporter.ReportEventCount(reportArgs, statusCode)
	writer.WriteHeader(statusCode)
}

func (h *Handler) sendEvent(ctx context.Context, headers http.Header, target string, event *cloudevents.Event, reporterArgs *ReportArgs, span *trace.Span) (*http.Response, error) {
	// Send the event to the subscriber
	req, err := h.sender.NewCloudEventRequestWithTarget(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create the request: %w", err)
	}

	message := binding.ToMessage(event)
	defer message.Finish(nil)

	additionalHeaders := utils.PassThroughHeaders(headers)
	err = kncloudevents.WriteHttpRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	start := time.Now()
	resp, err := h.sender.Send(req)
	dispatchTime := time.Since(start)
	if err != nil {
		err = fmt.Errorf("failed to dispatch message: %w", err)
	}

	sc := 0
	if resp != nil {
		sc = resp.StatusCode
	}

	_ = h.reporter.ReportEventDispatchTime(reporterArgs, sc, dispatchTime)

	return resp, err
}

func writeResponse(ctx context.Context, writer http.ResponseWriter, resp *http.Response, ttl int32, span *trace.Span) (int, error) {

	response := cehttp.NewMessageFromHttpResponse(resp)
	defer response.Finish(nil)

	if response.ReadEncoding() == binding.EncodingUnknown {
		// Response doesn't have a ce-specversion header nor a content-type matching a cloudevent event format
		// Just read a byte out of the reader to see if it's non-empty, we don't care what it is,
		// just that it is not empty. This means there was a response and it's not valid, so treat
		// as delivery failure.
		body := make([]byte, 1)
		n, _ := response.BodyReader.Read(body)
		response.BodyReader.Close()
		if n != 0 {
			return resp.StatusCode, errors.New("received a non-empty response not recognized as CloudEvent. The response MUST be or empty or a valid CloudEvent")
		}
		return resp.StatusCode, nil
	}

	event, err := binding.ToEvent(ctx, response)
	if err != nil {
		// Malformed event, reply with err
		return resp.StatusCode, err
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
	if err := broker.SetTTL(event.Context, ttl); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to reset TTL: %w", err)
	}

	eventResponse := binding.ToMessage(event)
	defer eventResponse.Finish(nil)

	if err := cehttp.WriteResponseWriter(ctx, eventResponse, resp.StatusCode, writer); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to write response event: %w", err)
	}

	return resp.StatusCode, nil
}

func (h *Handler) reportArrivalTime(event *event.Event, reportArgs *ReportArgs) {
	// Record the event processing time. This might be off if the receiver and the filter pods are running in
	// different nodes with different clocks.
	var arrivalTimeStr string
	if extErr := event.ExtensionAs(broker.EventArrivalTime, &arrivalTimeStr); extErr == nil {
		arrivalTime, err := time.Parse(time.RFC3339, arrivalTimeStr)
		if err == nil {
			_ = h.reporter.ReportEventProcessingTime(reportArgs, time.Since(arrivalTime))
		}
	}
}

func (h *Handler) getTrigger(ref path.NamespacedNameUID) (*eventingv1beta1.Trigger, error) {
	t, err := h.triggerLister.Triggers(ref.Namespace).Get(ref.Name)
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
func (h *Handler) shouldSendEvent(ctx context.Context, ts *eventingv1beta1.TriggerSpec, event *cloudevents.Event) FilterResult {
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
