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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	opencensusclient "github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	channelAttributes "knative.dev/eventing/pkg/channel/attributes"
	"knative.dev/pkg/logging"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	broker "knative.dev/eventing/pkg/broker"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/attributes"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
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

const (
	// NoResponse signals the step that send event to trigger's subscriber hasn't started
	NoResponse = -1
)

// ErrHandler handle the different errors of filter dispatch process
type ErrHandler struct {
	ResponseCode int
	ResponseBody []byte
	err          error
}

// HeaderProxyAllowList contains the headers that are proxied from the reply; other than the CloudEvents headers.
// Other headers are not proxied because of security concerns.
var HeaderProxyAllowList = map[string]struct{}{
	strings.ToLower("Retry-After"): {},
}

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
	withContext   func(ctx context.Context) context.Context
}

// NewHandler creates a new Handler and its associated MessageReceiver. The caller is responsible for
// Start()ing the returned Handler.
func NewHandler(logger *zap.Logger, triggerLister eventinglisters.TriggerLister, reporter StatsReporter, port int, wc func(ctx context.Context) context.Context) (*Handler, error) {
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
		withContext:   wc,
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

	ctx := h.withContext(request.Context())

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
	ctx = logging.WithLogger(ctx, h.logger.Sugar().With(zap.String("trigger", fmt.Sprintf("%s/%s", t.GetNamespace(), t.GetName()))))
	filterResult := filterEvent(ctx, t.Spec, *event)

	if filterResult == eventfilter.FailFilter {
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		return
	}

	h.reportArrivalTime(event, reportArgs)

	h.send(ctx, writer, request.Header, subscriberURI.URL(), reportArgs, event, ttl)
}

func (h *Handler) send(ctx context.Context, writer http.ResponseWriter, headers http.Header, target *url.URL, reportArgs *ReportArgs, event *cloudevents.Event, ttl int32) {
	// send the event to trigger's subscriber
	response, responseErr := h.sendEvent(ctx, headers, target, event, reportArgs)

	if responseErr.err != nil {
		h.logger.Error("failed to send event", zap.Error(responseErr.err))
		// If error is not because of the response, it should respond with http.StatusInternalServerError
		if responseErr.ResponseCode == NoResponse {

			writer.WriteHeader(http.StatusInternalServerError)
			_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
			return
		}
		// If error has a response propagate subscriber's headers back to channel
		if response != nil {
			proxyHeaders(response.Header, writer)
		}
		writer.WriteHeader(responseErr.ResponseCode)

		// Read Response body to responseErr
		errExtensionInfo := broker.ErrExtensionInfo{
			ErrDestination:  target,
			ErrResponseBody: responseErr.ResponseBody,
		}
		errExtensionBytes, msErr := json.Marshal(errExtensionInfo)
		if msErr != nil {
			h.logger.Error("failed to marshal errExtensionInfo", zap.Error(msErr))
			return
		}
		_, _ = writer.Write(errExtensionBytes)
		_ = h.reporter.ReportEventCount(reportArgs, responseErr.ResponseCode)

		return
	}

	h.logger.Debug("Successfully dispatched message", zap.Any("target", target.String()))

	// If there is an event in the response write it to the response
	statusCode, err := h.writeResponse(ctx, writer, response, ttl, target.String())
	if err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}
	_ = h.reporter.ReportEventCount(reportArgs, statusCode)
}

func (h *Handler) sendEvent(ctx context.Context, headers http.Header, target *url.URL, event *cloudevents.Event, reporterArgs *ReportArgs) (*http.Response, ErrHandler) {
	responseErr := ErrHandler{
		ResponseCode: NoResponse,
	}

	// Send the event to the subscriber
	req, err := h.sender.NewCloudEventRequestWithTarget(ctx, target.String())
	if err != nil {
		responseErr.err = fmt.Errorf("failed to create the request: %w", err)
		return nil, responseErr
	}

	message := binding.ToMessage(event)
	defer message.Finish(nil)

	additionalHeaders := utils.PassThroughHeaders(headers)

	// Following the spec https://github.com/knative/specs/blob/main/specs/eventing/data-plane.md#derived-reply-events
	additionalHeaders.Set("prefer", "reply")

	err = kncloudevents.WriteHTTPRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders)
	if err != nil {
		responseErr.err = fmt.Errorf("failed to write request: %w", err)
		return nil, responseErr
	}

	start := time.Now()
	resp, err := h.sender.Send(req)
	dispatchTime := time.Since(start)
	if err != nil {
		responseErr.ResponseCode = http.StatusInternalServerError
		responseErr.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", err.Error()))
		responseErr.err = fmt.Errorf("failed to dispatch message: %w", err)
		return resp, responseErr
	}

	sc := 0
	if resp != nil {
		sc = resp.StatusCode
		responseErr.ResponseCode = sc
	}

	_ = h.reporter.ReportEventDispatchTime(reporterArgs, sc, dispatchTime)

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		// Read response body into errHandler for failures
		body := make([]byte, channelAttributes.KnativeErrorDataExtensionMaxLength)

		readLen, readErr := resp.Body.Read(body)
		if readErr != nil && readErr != io.EOF {
			h.logger.Error("failed to read response body into DispatchExecutionInfo", zap.Error(readErr))
			responseErr.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", readErr.Error()))
		} else {
			responseErr.ResponseBody = body[:readLen]
		}
		responseErr.err = fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", resp.StatusCode)

		// Reject non-successful responses.
		return resp, responseErr
	}

	return resp, responseErr
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

func filterEvent(ctx context.Context, triggerSpec eventingv1.TriggerSpec, event cloudevents.Event) eventfilter.FilterResult {
	switch {
	case feature.FromContext(ctx).IsEnabled(feature.NewTriggerFilters) && len(triggerSpec.Filters) > 0:
		logging.FromContext(ctx).Debugw("New trigger filters feature is enabled. Applying new filters.", zap.Any("filters", triggerSpec.Filters))
		return applySubscriptionsAPIFilters(ctx, triggerSpec.Filters, event)
	case triggerSpec.Filter != nil:
		logging.FromContext(ctx).Debugw("Applying attributes filter.", zap.Any("filter", triggerSpec.Filter))
		return applyAttributesFilter(ctx, triggerSpec.Filter, event)
	default:
		logging.FromContext(ctx).Debugw("Found no filters in trigger", zap.Any("triggerSpec", triggerSpec))
		return eventfilter.NoFilter
	}
}

func applySubscriptionsAPIFilters(ctx context.Context, filters []eventingv1.SubscriptionsAPIFilter, event cloudevents.Event) eventfilter.FilterResult {
	return subscriptionsapi.NewAllFilter(materializeFiltersList(ctx, filters)...).Filter(ctx, event)
}

func materializeSubscriptionsAPIFilter(ctx context.Context, filter eventingv1.SubscriptionsAPIFilter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewExactFilter(filter.Exact)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid exact expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewPrefixFilter(filter.Prefix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid prefix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewSuffixFilter(filter.Suffix)
		if err != nil {
			logging.FromContext(ctx).Debugw("Invalid suffix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.All) > 0:
		materializedFilter = subscriptionsapi.NewAllFilter(materializeFiltersList(ctx, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = subscriptionsapi.NewAnyFilter(materializeFiltersList(ctx, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = subscriptionsapi.NewNotFilter(materializeSubscriptionsAPIFilter(ctx, *filter.Not))
	case filter.CESQL != "":
		if materializedFilter, err = subscriptionsapi.NewCESQLFilter(filter.CESQL); err != nil {
			// This is weird, CESQL expression should be validated when Trigger's are created.
			logging.FromContext(ctx).Debugw("Found an Invalid CE SQL expression", zap.String("expression", filter.CESQL))
			return nil
		}
	}
	return materializedFilter
}

func materializeFiltersList(ctx context.Context, filters []eventingv1.SubscriptionsAPIFilter) []eventfilter.Filter {
	materializedFilters := make([]eventfilter.Filter, 0, len(filters))
	for _, f := range filters {
		f := materializeSubscriptionsAPIFilter(ctx, f)
		if f == nil {
			logging.FromContext(ctx).Warnw("Failed to parse filter. Skipping filter.", zap.Any("filter", f))
			continue
		}
		materializedFilters = append(materializedFilters, f)
	}
	return materializedFilters
}

func applyAttributesFilter(ctx context.Context, filter *eventingv1.TriggerFilter, event cloudevents.Event) eventfilter.FilterResult {
	return attributes.NewAttributesFilter(filter.Attributes).Filter(ctx, event)
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
		// *Only* proxy some headers because of security reasons
		if isInProxyHeaderAllowList(headerKey) {
			for _, headerValue := range headerValues {
				writer.Header().Add(headerKey, headerValue)
			}
		}
	}
}

func isInProxyHeaderAllowList(headerKey string) bool {
	_, exists := HeaderProxyAllowList[strings.ToLower(headerKey)]
	return exists
}
