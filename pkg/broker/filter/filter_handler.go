/*
Copyright 2019 The Knative Authors

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

package filter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	opencensusclient "github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	broker "knative.dev/eventing/pkg/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/attributes"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/reconciler/sugar/trigger/path"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const (
	// TODO make these constants configurable (either as env variables, config map, or part of broker spec).
	//  Issue: https://github.com/knative/eventing/issues/1777
	// Constants for the underlying HTTP Client transport. These would enable better connection reuse.
	// Set them on a 10:1 ratio, but this would actually depend on the Triggers' subscribers and the workload itself.
	// These are magic numbers, partly set based on empirical evidence running performance workloads, and partly
	// based on what serving is doing. See https://github.com/knative/serving/blob/main/pkg/network/transports.go.
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 100
)

// Handler parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Handler struct {
	// receiver receives incoming HTTP requests
	receiver *kncloudevents.HTTPMessageReceiver
	// sender sends requests to downstream services
	sender *kncloudevents.HTTPMessageSender
	// reporter reports stats of status code and dispatch time
	reporter StatsReporter

	triggerLister eventinglisters.TriggerLister
	logger        *zap.Logger
}

// NewHandler creates a new Handler and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned Handler.
func NewHandler(logger *zap.Logger, triggerLister eventinglisters.TriggerLister, reporter StatsReporter, port int) (*Handler, error) {
	kncloudevents.ConfigureConnectionArgs(&kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	})

	sender, err := kncloudevents.NewHTTPMessageSenderWithTarget("")
	if err != nil {
		return nil, fmt.Errorf("failed to create message sender: %w", err)
	}

	return &Handler{
		receiver:      kncloudevents.NewHTTPMessageReceiver(port),
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
	return h.receiver.StartListen(ctx, h)
}

// 1. validate request
// 2. extract event from request
// 3. get trigger from its trigger reference extracted from the request URI
// 4. filter event
// 5. send event to trigger's subscriber
// 6. write the response
func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Allow", "POST")

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
		span.AddAttributes(opencensusclient.EventTraceAttributes(event)...)
	}

	// Remove the TTL attribute that is used by the Broker.
	ttl, err := broker.GetTTL(event.Context)
	if err != nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		h.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// Return a BadRequest error, so the upstream can decide how to handle it, e.g. sending
		// the message to a Dead Letter Sink.
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
	filterResult := filterEvent(ctx, t.Spec.Filter, *event)

	if filterResult == eventfilter.FailFilter {
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		return
	}

	h.reportArrivalTime(event, reportArgs)

	h.send(ctx, writer, request.Header, subscriberURI.String(), reportArgs, event, ttl)
}

func (h *Handler) send(ctx context.Context, writer http.ResponseWriter, headers http.Header, target string, reportArgs *ReportArgs, event *cloudevents.Event, ttl int32) {
	// send the event to trigger's subscriber
	response, err := h.sendEvent(ctx, headers, target, event, reportArgs)
	if err != nil {
		h.logger.Error("failed to send event", zap.Error(err))
		writer.WriteHeader(http.StatusInternalServerError)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
		return
	}

	h.logger.Debug("Successfully dispatched message", zap.Any("target", target))

	// If there is an event in the response write it to the response
	statusCode, err := h.writeResponse(ctx, writer, response, ttl, target)
	if err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}
	_ = h.reporter.ReportEventCount(reportArgs, statusCode)
}

func (h *Handler) sendEvent(ctx context.Context, headers http.Header, target string, event *cloudevents.Event, reporterArgs *ReportArgs) (*http.Response, error) {
	// Send the event to the subscriber
	req, err := h.sender.NewCloudEventRequestWithTarget(ctx, target)
	if err != nil {
		return nil, fmt.Errorf("failed to create the request: %w", err)
	}

	message := binding.ToMessage(event)
	defer message.Finish(nil)

	additionalHeaders := utils.PassThroughHeaders(headers)

	// Following the spec https://github.com/knative/specs/blob/main/specs/eventing/data-plane.md#derived-reply-events
	additionalHeaders.Set("prefer", "reply")

	err = kncloudevents.WriteHTTPRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders)
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

// The return values are the status
func (h *Handler) writeResponse(ctx context.Context, writer http.ResponseWriter, resp *http.Response, ttl int32, target string) (int, error) {
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
			// Note that we could just use StatusInternalServerError, but to distinguish
			// between the failure cases, we use a different code here.
			writer.WriteHeader(http.StatusBadGateway)
			return http.StatusBadGateway, errors.New("received a non-empty response not recognized as CloudEvent. The response MUST be either empty or a valid CloudEvent")
		}
		proxyHeaders(resp.Header, writer) // Proxy original Response Headers for downstream use
		h.logger.Debug("Response doesn't contain a CloudEvent, replying with an empty response", zap.Any("target", target))
		writer.WriteHeader(resp.StatusCode)
		return resp.StatusCode, nil
	}

	event, err := binding.ToEvent(ctx, response)
	if err != nil {
		// Like in the above case, we could just use StatusInternalServerError, but to distinguish
		// between the failure cases, we use a different code here.
		writer.WriteHeader(http.StatusBadGateway)
		// Malformed event, reply with err
		return http.StatusBadGateway, err
	}

	// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
	if err := broker.SetTTL(event.Context, ttl); err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return http.StatusInternalServerError, fmt.Errorf("failed to reset TTL: %w", err)
	}

	eventResponse := binding.ToMessage(event)
	defer eventResponse.Finish(nil)

	// Proxy the original Response Headers for downstream use
	proxyHeaders(resp.Header, writer)

	if err := cehttp.WriteResponseWriter(ctx, eventResponse, resp.StatusCode, writer); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to write response event: %w", err)
	}

	h.logger.Debug("Replied with a CloudEvent response", zap.Any("target", target))

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

func (h *Handler) getTrigger(ref path.NamespacedNameUID) (*eventingv1.Trigger, error) {
	t, err := h.triggerLister.Triggers(ref.Namespace).Get(ref.Name)
	if err != nil {
		return nil, err
	}
	if t.UID != ref.UID {
		return nil, fmt.Errorf("trigger had a different UID. From ref '%s'. From Kubernetes '%s'", ref.UID, t.UID)
	}
	return t, nil
}

func filterEvent(ctx context.Context, filter *eventingv1.TriggerFilter, event cloudevents.Event) eventfilter.FilterResult {
	if filter == nil {
		return eventfilter.NoFilter
	}
	var filters eventfilter.Filters
	if filter.Attributes != nil && len(filter.Attributes) != 0 {
		filters = append(filters, attributes.NewAttributesFilter(filter.Attributes))
	}
	return filters.Filter(ctx, event)
}

// triggerFilterAttribute returns the filter attribute value for a given `attributeName`. If it doesn't not exist,
// returns the any value filter.
func triggerFilterAttribute(filter *eventingv1.TriggerFilter, attributeName string) string {
	attributeValue := eventingv1.TriggerAnyFilter
	if filter != nil {
		attrs := map[string]string(filter.Attributes)
		if v, ok := attrs[attributeName]; ok {
			attributeValue = v
		}
	}
	return attributeValue
}

// proxyHeaders adds the specified HTTP Headers to the ResponseWriter.
func proxyHeaders(httpHeader http.Header, writer http.ResponseWriter) {
	for headerKey, headerValues := range httpHeader {
		for _, headerValue := range headerValues {
			writer.Header().Add(headerKey, headerValue)
		}
	}
}
