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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	opencensusclient "github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/apis"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/utils"

	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	eventingbroker "knative.dev/eventing/pkg/broker"
	v1 "knative.dev/eventing/pkg/client/informers/externalversions/eventing/v1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/eventfilter"
	"knative.dev/eventing/pkg/eventfilter/attributes"
	"knative.dev/eventing/pkg/eventfilter/subscriptionsapi"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/reconciler/sugar/trigger/path"
	"knative.dev/eventing/pkg/tracing"
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

	FilterAudience = "mt-broker-filter"
	skipTTL        = -1
)

// Handler parses Cloud Events, determines if they pass a filter, and sends them to a subscriber.
type Handler struct {
	// reporter reports stats of status code and dispatch time
	reporter StatsReporter

	eventDispatcher *kncloudevents.Dispatcher

	triggerLister    eventinglisters.TriggerLister
	brokerLister     eventinglisters.BrokerLister
	logger           *zap.Logger
	withContext      func(ctx context.Context) context.Context
	filtersMap       *subscriptionsapi.FiltersMap
	tokenVerifier    *auth.OIDCTokenVerifier
	EventTypeCreator *eventtype.EventTypeAutoHandler
}

// NewHandler creates a new Handler and its associated EventReceiver.
func NewHandler(logger *zap.Logger, tokenVerifier *auth.OIDCTokenVerifier, oidcTokenProvider *auth.OIDCTokenProvider, triggerInformer v1.TriggerInformer, brokerInformer v1.BrokerInformer, reporter StatsReporter, trustBundleConfigMapLister corev1listers.ConfigMapNamespaceLister, wc func(ctx context.Context) context.Context) (*Handler, error) {
	kncloudevents.ConfigureConnectionArgs(&kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	})

	fm := subscriptionsapi.NewFiltersMap()

	clientConfig := eventingtls.ClientConfig{
		TrustBundleConfigMapLister: trustBundleConfigMapLister,
	}

	triggerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			trigger, ok := obj.(*eventingv1.Trigger)
			if !ok {
				return
			}
			logger.Debug("Adding filter to filtersMap")
			fm.Set(trigger, createSubscriptionsAPIFilters(logger, trigger))
			kncloudevents.AddOrUpdateAddressableHandler(clientConfig, duckv1.Addressable{
				URL:     trigger.Status.SubscriberURI,
				CACerts: trigger.Status.SubscriberCACerts,
			})
		},
		UpdateFunc: func(_, obj interface{}) {
			trigger, ok := obj.(*eventingv1.Trigger)
			if !ok {
				return
			}
			logger.Debug("Updating filter in filtersMap")
			fm.Set(trigger, createSubscriptionsAPIFilters(logger, trigger))
			kncloudevents.AddOrUpdateAddressableHandler(clientConfig, duckv1.Addressable{
				URL:     trigger.Status.SubscriberURI,
				CACerts: trigger.Status.SubscriberCACerts,
			})
		},
		DeleteFunc: func(obj interface{}) {
			trigger, ok := obj.(*eventingv1.Trigger)
			if !ok {
				return
			}
			logger.Debug("Deleting filter in filtersMap")
			fm.Delete(trigger)
			kncloudevents.DeleteAddressableHandler(duckv1.Addressable{
				URL:     trigger.Status.SubscriberURI,
				CACerts: trigger.Status.SubscriberCACerts,
			})
		},
	})

	return &Handler{
		reporter:        reporter,
		eventDispatcher: kncloudevents.NewDispatcher(clientConfig, oidcTokenProvider),
		triggerLister:   triggerInformer.Lister(),
		brokerLister:    brokerInformer.Lister(),
		logger:          logger,
		tokenVerifier:   tokenVerifier,
		withContext:     wc,
		filtersMap:      fm,
	}, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx := h.withContext(request.Context())

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

	trigger, err := h.getTrigger(triggerRef)
	if err != nil {
		h.logger.Info("Unable to get the Trigger", zap.Error(err), zap.Any("triggerRef", triggerRef))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	event, err := cehttp.NewEventFromHTTPRequest(request)
	if err != nil {
		h.logger.Warn("failed to extract event from request", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	h.logger.Debug("Received message", zap.Any("trigger", triggerRef.NamespacedName), zap.Stringer("event", event))

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

	features := feature.FromContext(ctx)
	if features.IsOIDCAuthentication() {
		h.logger.Debug("OIDC authentication is enabled")

		audience := FilterAudience

		err = h.tokenVerifier.VerifyJWTFromRequest(ctx, request, &audience, writer)
		if err != nil {
			h.logger.Warn("Error when validating the JWT token in the request", zap.Error(err))
			return
		}

		h.logger.Debug("Request contained a valid JWT. Continuing...")
	}

	if triggerRef.IsReply {
		h.handleDispatchToReplyRequest(ctx, trigger, writer, request, event)
		return
	}

	if triggerRef.IsDLS {
		h.handleDispatchToDLSRequest(ctx, trigger, writer, request, event)
		return
	}

	h.handleDispatchToSubscriberRequest(ctx, trigger, writer, request, event)
}

func (h *Handler) handleDispatchToReplyRequest(ctx context.Context, trigger *eventingv1.Trigger, writer http.ResponseWriter, request *http.Request, event *event.Event) {
	broker, err := h.brokerLister.Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
	if err != nil {
		h.logger.Info("Unable to get the Broker", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	target := broker.Status.Address

	reportArgs := &ReportArgs{
		ns:          trigger.Namespace,
		trigger:     trigger.Name,
		broker:      trigger.Spec.Broker,
		requestType: "reply_forward",
	}

	if request.TLS != nil {
		reportArgs.requestScheme = "https"
	} else {
		reportArgs.requestScheme = "http"
	}

	h.logger.Info("sending to reply", zap.Any("target", target))

	// since the broker-filter acts here like a proxy, we don't filter headers
	h.send(ctx, writer, request.Header, *target, reportArgs, event, trigger, skipTTL)
}

func (h *Handler) handleDispatchToDLSRequest(ctx context.Context, trigger *eventingv1.Trigger, writer http.ResponseWriter, request *http.Request, event *event.Event) {
	broker, err := h.brokerLister.Brokers(trigger.Namespace).Get(trigger.Spec.Broker)
	if err != nil {
		h.logger.Info("Unable to get the Broker", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	var target *duckv1.Addressable
	if trigger.Status.DeadLetterSinkURI != nil {
		target = &duckv1.Addressable{
			URL:      trigger.Status.DeadLetterSinkURI,
			CACerts:  trigger.Status.DeadLetterSinkCACerts,
			Audience: trigger.Status.DeadLetterSinkAudience,
		}
	} else if broker.Status.DeadLetterSinkURI != nil {
		target = &duckv1.Addressable{
			URL:      broker.Status.DeadLetterSinkURI,
			CACerts:  broker.Status.DeadLetterSinkCACerts,
			Audience: broker.Status.DeadLetterSinkAudience,
		}
	}

	reportArgs := &ReportArgs{
		ns:          trigger.Namespace,
		trigger:     trigger.Name,
		broker:      trigger.Spec.Broker,
		requestType: "dls_forward",
	}

	if request.TLS != nil {
		reportArgs.requestScheme = "https"
	} else {
		reportArgs.requestScheme = "http"
	}

	h.logger.Info("sending to dls", zap.Any("target", target))

	// since the broker-filter acts here like a proxy, we don't filter headers
	h.send(ctx, writer, request.Header, *target, reportArgs, event, trigger, skipTTL)
}

func (h *Handler) handleDispatchToSubscriberRequest(ctx context.Context, trigger *eventingv1.Trigger, writer http.ResponseWriter, request *http.Request, event *event.Event) {
	triggerRef := types.NamespacedName{
		Name:      trigger.Name,
		Namespace: trigger.Namespace,
	}

	// Remove the TTL attribute that is used by the Broker.
	ttl, err := eventingbroker.GetTTL(event.Context)
	if err != nil {
		// Only messages sent by the Broker should be here. If the attribute isn't here, then the
		// event wasn't sent by the Broker, so we can drop it.
		h.logger.Warn("No TTL seen, dropping", zap.Any("triggerRef", triggerRef), zap.Any("event", event))
		// Return a BadRequest error, so the upstream can decide how to handle it, e.g. sending
		// the message to a Dead Letter Sink.
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := eventingbroker.DeleteTTL(event.Context); err != nil {
		h.logger.Warn("Failed to delete TTL.", zap.Error(err))
	}

	reportArgs := &ReportArgs{
		ns:          trigger.Namespace,
		trigger:     trigger.Name,
		broker:      trigger.Spec.Broker,
		filterType:  triggerFilterAttribute(trigger.Spec.Filter, "type"),
		requestType: "filter",
	}

	if request.TLS != nil {
		reportArgs.requestScheme = "https"
	} else {
		reportArgs.requestScheme = "http"
	}

	subscriberURI := trigger.Status.SubscriberURI
	if subscriberURI == nil {
		// Record the event count.
		writer.WriteHeader(http.StatusBadRequest)
		_ = h.reporter.ReportEventCount(reportArgs, http.StatusBadRequest)
		return
	}

	// Check if the event should be sent.
	ctx = logging.WithLogger(ctx, h.logger.Sugar().With(zap.String("trigger", fmt.Sprintf("%s/%s", trigger.GetNamespace(), trigger.GetName()))))
	filterResult := h.filterEvent(ctx, trigger, *event)

	if filterResult == eventfilter.FailFilter {
		// We do not count the event. The event will be counted in the broker ingress.
		// If the filter didn't pass, it means that the event wasn't meant for this Trigger.
		return
	}

	h.reportArrivalTime(event, reportArgs)

	target := duckv1.Addressable{
		URL:      trigger.Status.SubscriberURI,
		CACerts:  trigger.Status.SubscriberCACerts,
		Audience: trigger.Status.SubscriberAudience,
	}

	h.send(ctx, writer, utils.PassThroughHeaders(request.Header), target, reportArgs, event, trigger, ttl)
}

func (h *Handler) send(ctx context.Context, writer http.ResponseWriter, headers http.Header, target duckv1.Addressable, reportArgs *ReportArgs, event *cloudevents.Event, t *eventingv1.Trigger, ttl int32) {
	additionalHeaders := headers.Clone()
	additionalHeaders.Set(apis.KnNamespaceHeader, t.GetNamespace())

	opts := []kncloudevents.SendOption{
		kncloudevents.WithHeader(additionalHeaders),
	}

	if h.EventTypeCreator != nil {
		opts = append(opts, kncloudevents.WithEventTypeAutoHandler(
			h.EventTypeCreator,
			&duckv1.KReference{
				Name:       t.Name,
				Namespace:  t.Namespace,
				APIVersion: eventingv1.SchemeGroupVersion.String(),
				Kind:       "Trigger",
			},
			t.UID,
		))
	}

	if t.Status.Auth != nil && t.Status.Auth.ServiceAccountName != nil {
		opts = append(opts, kncloudevents.WithOIDCAuthentication(&types.NamespacedName{
			Name:      *t.Status.Auth.ServiceAccountName,
			Namespace: t.Namespace,
		}))
	}

	dispatchInfo, err := h.eventDispatcher.SendEvent(ctx, *event, target, opts...)
	if err != nil {
		h.logger.Error("failed to send event", zap.Error(err))

		// If error is not because of the response, it should respond with http.StatusInternalServerError
		if dispatchInfo.ResponseCode <= 0 {
			writer.WriteHeader(http.StatusInternalServerError)
			_ = h.reporter.ReportEventCount(reportArgs, http.StatusInternalServerError)
			return
		}

		h.reporter.ReportEventDispatchTime(reportArgs, dispatchInfo.ResponseCode, dispatchInfo.Duration)

		writeHeaders(utils.PassThroughHeaders(dispatchInfo.ResponseHeader), writer)
		writer.WriteHeader(dispatchInfo.ResponseCode)

		// Read Response body to responseErr
		errExtensionInfo := eventingbroker.ErrExtensionInfo{
			ErrDestination:  target.URL,
			ErrResponseBody: dispatchInfo.ResponseBody,
		}
		errExtensionBytes, msErr := json.Marshal(errExtensionInfo)
		if msErr != nil {
			h.logger.Error("failed to marshal errExtensionInfo", zap.Error(msErr))
			return
		}
		_, err = writer.Write(errExtensionBytes)
		if err != nil {
			h.logger.Error("failed to write error response", zap.Error(err))
		}
		_ = h.reporter.ReportEventCount(reportArgs, dispatchInfo.ResponseCode)

		return
	}

	h.logger.Debug("Successfully dispatched message", zap.Any("target", target))

	h.reporter.ReportEventDispatchTime(reportArgs, dispatchInfo.ResponseCode, dispatchInfo.Duration)

	// If there is an event in the response write it to the response
	statusCode, err := h.writeResponse(ctx, writer, dispatchInfo, ttl, target.URL.String())
	if err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}
	_ = h.reporter.ReportEventCount(reportArgs, statusCode)
}

// The return values are the status
func (h *Handler) writeResponse(ctx context.Context, writer http.ResponseWriter, dispatchInfo *kncloudevents.DispatchInfo, ttl int32, target string) (int, error) {
	response := cehttp.NewMessage(dispatchInfo.ResponseHeader, io.NopCloser(bytes.NewReader(dispatchInfo.ResponseBody)))
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

		writeHeaders(utils.PassThroughHeaders(dispatchInfo.ResponseHeader), writer) // Proxy original Response Headers for downstream use
		h.logger.Debug("Response doesn't contain a CloudEvent, replying with an empty response", zap.Any("target", target))
		writer.WriteHeader(dispatchInfo.ResponseCode)
		return dispatchInfo.ResponseCode, nil
	}

	event, err := binding.ToEvent(ctx, response)
	if err != nil {
		// Like in the above case, we could just use StatusInternalServerError, but to distinguish
		// between the failure cases, we use a different code here.
		writer.WriteHeader(http.StatusBadGateway)
		// Malformed event, reply with err
		return http.StatusBadGateway, err
	}

	if ttl != skipTTL {
		// Reattach the TTL (with the same value) to the response event before sending it to the Broker.
		if err := eventingbroker.SetTTL(event.Context, ttl); err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return http.StatusInternalServerError, fmt.Errorf("failed to reset TTL: %w", err)
		}
	}

	eventResponse := binding.ToMessage(event)
	defer eventResponse.Finish(nil)

	// Proxy the original Response Headers for downstream use
	writeHeaders(utils.PassThroughHeaders(dispatchInfo.ResponseHeader), writer)

	if err := cehttp.WriteResponseWriter(ctx, eventResponse, dispatchInfo.ResponseCode, writer); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to write response event: %w", err)
	}

	h.logger.Debug("Replied with a CloudEvent response", zap.Any("target", target))

	return dispatchInfo.ResponseCode, nil
}

func (h *Handler) reportArrivalTime(event *event.Event, reportArgs *ReportArgs) {
	// Record the event processing time. This might be off if the receiver and the filter pods are running in
	// different nodes with different clocks.
	var arrivalTimeStr string
	if extErr := event.ExtensionAs(eventingbroker.EventArrivalTime, &arrivalTimeStr); extErr == nil {
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

func (h *Handler) filterEvent(ctx context.Context, trigger *eventingv1.Trigger, event cloudevents.Event) eventfilter.FilterResult {
	switch {
	case feature.FromContext(ctx).IsEnabled(feature.NewTriggerFilters) && len(trigger.Spec.Filters) > 0:
		logging.FromContext(ctx).Debugw("New trigger filters feature is enabled. Applying new filters.", zap.Any("filters", trigger.Spec.Filters))
		filter, ok := h.filtersMap.Get(trigger)
		if !ok {
			// trigger filters haven't updated in the map yet - need to create them on the fly
			return applySubscriptionsAPIFilters(ctx, trigger, event)
		}
		return filter.Filter(ctx, event)
	case trigger.Spec.Filter != nil:
		logging.FromContext(ctx).Debugw("Applying attributes filter.", zap.Any("filter", trigger.Spec.Filter))
		return applyAttributesFilter(ctx, trigger.Spec.Filter, event)
	default:
		logging.FromContext(ctx).Debugw("Found no filters in trigger", zap.Any("triggerSpec", trigger.Spec))
		return eventfilter.NoFilter
	}
}

func applySubscriptionsAPIFilters(ctx context.Context, trigger *eventingv1.Trigger, event cloudevents.Event) eventfilter.FilterResult {
	return createSubscriptionsAPIFilters(logging.FromContext(ctx).Desugar(), trigger).Filter(ctx, event)
}

func createSubscriptionsAPIFilters(logger *zap.Logger, trigger *eventingv1.Trigger) eventfilter.Filter {
	if len(trigger.Spec.Filters) == 0 {
		logger.Debug("Found no filters for trigger", zap.Any("trigger.Spec", trigger.Spec))
		return subscriptionsapi.NewNoFilter()
	}
	return subscriptionsapi.NewAllFilter(MaterializeFiltersList(logger, trigger.Spec.Filters)...)
}

func materializeSubscriptionsAPIFilter(logger *zap.Logger, filter eventingv1.SubscriptionsAPIFilter) eventfilter.Filter {
	var materializedFilter eventfilter.Filter
	var err error
	switch {
	case len(filter.Exact) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewExactFilter(filter.Exact)
		if err != nil {
			logger.Debug("Invalid exact expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Prefix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewPrefixFilter(filter.Prefix)
		if err != nil {
			logger.Debug("Invalid prefix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.Suffix) > 0:
		// The webhook validates that this map has only a single key:value pair.
		materializedFilter, err = subscriptionsapi.NewSuffixFilter(filter.Suffix)
		if err != nil {
			logger.Debug("Invalid suffix expression", zap.Any("filters", filter.Exact), zap.Error(err))
			return nil
		}
	case len(filter.All) > 0:
		materializedFilter = subscriptionsapi.NewAllFilter(MaterializeFiltersList(logger, filter.All)...)
	case len(filter.Any) > 0:
		materializedFilter = subscriptionsapi.NewAnyFilter(MaterializeFiltersList(logger, filter.Any)...)
	case filter.Not != nil:
		materializedFilter = subscriptionsapi.NewNotFilter(materializeSubscriptionsAPIFilter(logger, *filter.Not))
	case filter.CESQL != "":
		if materializedFilter, err = subscriptionsapi.NewCESQLFilter(filter.CESQL); err != nil {
			// This is weird, CESQL expression should be validated when Trigger's are created.
			logger.Debug("Found an Invalid CE SQL expression", zap.String("expression", filter.CESQL))
			return nil
		}
	}
	return materializedFilter
}

// MaterialzieFilterList allows any component that supports `SubscriptionsAPIFilter` to process them
func MaterializeFiltersList(logger *zap.Logger, filters []eventingv1.SubscriptionsAPIFilter) []eventfilter.Filter {
	materializedFilters := make([]eventfilter.Filter, 0, len(filters))
	for _, f := range filters {
		f := materializeSubscriptionsAPIFilter(logger, f)
		if f == nil {
			logger.Warn("Failed to parse filter. Skipping filter.", zap.Any("filter", f))
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

// writeHeaders adds the specified HTTP Headers to the ResponseWriter.
func writeHeaders(httpHeader http.Header, writer http.ResponseWriter) {
	for headerKey, headerValues := range httpHeader {
		for _, headerValue := range headerValues {
			writer.Header().Add(headerKey, headerValue)
		}
	}
}
