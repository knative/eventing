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

package ingress

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/apis/eventing"
	eventingv1 "knative.dev/eventing/pkg/apis/eventing/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/broker"
	v1 "knative.dev/eventing/pkg/client/informers/externalversions/eventing/v1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
	ScopeName                        = "knative.dev/pkg/broker/ingress"
)

var (
	latencyBounds = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
)

type Handler struct {
	// Defaults sets default values to incoming events
	Defaulter client.EventDefaulter
	// BrokerLister gets broker objects
	BrokerLister     eventinglisters.BrokerLister
	EvenTypeHandler  *eventtype.EventTypeAutoHandler
	Logger           *zap.Logger
	eventDispatcher  *kncloudevents.Dispatcher
	tokenVerifier    *auth.Verifier
	withContext      func(ctx context.Context) context.Context
	tracer           trace.Tracer
	dispatchDuration metric.Float64Histogram
}

func NewHandler(
	logger *zap.Logger,
	defaulter client.EventDefaulter,
	brokerInformer v1.BrokerInformer,
	tokenVerifier *auth.Verifier,
	oidcTokenProvider *auth.OIDCTokenProvider,
	trustBundleConfigMapLister corev1listers.ConfigMapNamespaceLister,
	withContext func(ctx context.Context) context.Context,
	meterProvider metric.MeterProvider,
	traceProvider trace.TracerProvider,
) (*Handler, error) {
	connectionArgs := kncloudevents.ConnectionArgs{
		MaxIdleConns:        defaultMaxIdleConnections,
		MaxIdleConnsPerHost: defaultMaxIdleConnectionsPerHost,
	}
	kncloudevents.ConfigureConnectionArgs(&connectionArgs)

	clientConfig := eventingtls.ClientConfig{
		TrustBundleConfigMapLister: trustBundleConfigMapLister,
	}

	brokerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			broker, ok := obj.(*eventingv1.Broker)
			if !ok || broker == nil || broker.Status.Address == nil {
				return
			}
			kncloudevents.AddOrUpdateAddressableHandler(clientConfig, duckv1.Addressable{
				URL:     broker.Status.Address.URL,
				CACerts: broker.Status.Address.CACerts,
			}, meterProvider, traceProvider)
		},
		UpdateFunc: func(_, obj interface{}) {
			broker, ok := obj.(*eventingv1.Broker)
			if !ok || broker == nil || broker.Status.Address == nil {
				return
			}
			kncloudevents.AddOrUpdateAddressableHandler(clientConfig, duckv1.Addressable{
				URL:     broker.Status.Address.URL,
				CACerts: broker.Status.Address.CACerts,
			}, meterProvider, traceProvider)
		},
		DeleteFunc: func(obj interface{}) {
			broker, ok := obj.(*eventingv1.Broker)
			if !ok || broker == nil || broker.Status.Address == nil {
				return
			}
			kncloudevents.DeleteAddressableHandler(duckv1.Addressable{
				URL:     broker.Status.Address.URL,
				CACerts: broker.Status.Address.CACerts,
			})
		},
	})

	h := &Handler{
		Defaulter:    defaulter,
		Logger:       logger,
		BrokerLister: brokerInformer.Lister(),
		eventDispatcher: kncloudevents.NewDispatcher(
			clientConfig,
			oidcTokenProvider,
			kncloudevents.WithMeterProvider(meterProvider),
			kncloudevents.WithTraceProvider(traceProvider),
		),
		tokenVerifier: tokenVerifier,
		withContext:   withContext,
		tracer:        traceProvider.Tracer(ScopeName),
	}

	meter := meterProvider.Meter(ScopeName)

	var err error
	h.dispatchDuration, err = meter.Float64Histogram(
		"kn.eventing.dispatch.duration",
		metric.WithDescription("The duration to dispatch the event"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(latencyBounds...),
	)
	if err != nil {
		return nil, err
	}

	return h, nil
}

func (h *Handler) getBroker(name, namespace string) (*eventingv1.Broker, error) {
	broker, err := h.BrokerLister.Brokers(namespace).Get(name)
	if err != nil {
		h.Logger.Warn("Broker getter failed")
		return nil, err
	}
	return broker, nil
}

func (h *Handler) getChannelAddress(broker *eventingv1.Broker) (*duckv1.Addressable, error) {
	if broker.Status.Annotations == nil {
		return nil, fmt.Errorf("broker status annotations uninitialized")
	}
	address, present := broker.Status.Annotations[eventing.BrokerChannelAddressStatusAnnotationKey]
	if !present {
		return nil, fmt.Errorf("channel address not found in broker status annotations")
	}

	url, err := apis.ParseURL(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse channel address url")
	}

	addr := &duckv1.Addressable{
		URL: url,
	}

	if certs, ok := broker.Status.Annotations[eventing.BrokerChannelCACertsStatusAnnotationKey]; ok && certs != "" {
		addr.CACerts = &certs
	}

	if audience, ok := broker.Status.Annotations[eventing.BrokerChannelAudienceStatusAnnotationKey]; ok && audience != "" {
		addr.Audience = &audience
	}

	return addr, nil
}

func (h *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Allow", "POST, OPTIONS")
	// validate request method
	if request.Method == http.MethodOptions {
		writer.Header().Set("WebHook-Allowed-Origin", "*") // Accept from any Origin:
		writer.Header().Set("WebHook-Allowed-Rate", "*")   // Unlimited requests/minute
		writer.WriteHeader(http.StatusOK)
		return
	}
	if request.Method != http.MethodPost {
		h.Logger.Warn("unexpected request method", zap.String("method", request.Method))
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// validate request URI
	if request.RequestURI == "/" {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	nsBrokerName := strings.Split(strings.TrimSuffix(request.RequestURI, "/"), "/")
	if len(nsBrokerName) != 3 {
		h.Logger.Info("Malformed uri", zap.String("URI", request.RequestURI))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	ctx := h.withContext(request.Context())
	ctx = observability.WithRequestLabels(ctx, request)

	message := cehttp.NewMessageFromHttpRequest(request)
	defer message.Finish(nil)

	event, err := binding.ToEvent(ctx, message)
	if err != nil {
		h.Logger.Warn("failed to extract event from request", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	// run validation for the extracted event
	validationErr := event.Validate()
	if validationErr != nil {
		h.Logger.Warn("failed to validate extracted event", zap.Error(validationErr))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	brokerNamespace := nsBrokerName[1]
	brokerName := nsBrokerName[2]
	brokerNamespacedName := types.NamespacedName{
		Name:      brokerName,
		Namespace: brokerNamespace,
	}

	broker, err := h.getBroker(brokerName, brokerNamespace)
	if apierrors.IsNotFound(err) {
		h.Logger.Warn("Failed to retrieve broker", zap.Error(err))
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		h.Logger.Warn("Failed to retrieve broker", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	features := feature.FromContext(ctx)
	audience := ptr.To("")
	if broker.Status.Address != nil {
		audience = broker.Status.Address.Audience
	}
	err = h.tokenVerifier.VerifyRequest(ctx, features, audience, brokerNamespace, broker.Status.Policies, request, writer)
	if err != nil {
		h.Logger.Warn("Failed to verify AuthN and AuthZ.", zap.Error(err))
		return
	}

	ctx = observability.WithBrokerLabels(ctx, brokerNamespacedName)
	ctx = observability.WithMinimalEventLabels(ctx, event)

	ctx, span := h.tracer.Start(ctx, tracing.BrokerMessagingDestination(brokerNamespacedName))
	defer func() {
		if span.IsRecording() {
			// add event labels here so that they only populate the span, and not any metrics
			ctx = observability.WithEventLabels(ctx, event)
			labeler, _ := otelhttp.LabelerFromContext(ctx)
			span.SetAttributes(labeler.Get()...)
		}

		span.End()
	}()

	statusCode, dispatchTime := h.receive(ctx, utils.PassThroughHeaders(request.Header), event, broker)
	if dispatchTime > kncloudevents.NoDuration {
		labeler, _ := otelhttp.LabelerFromContext(ctx)
		h.dispatchDuration.Record(ctx, dispatchTime.Seconds(), metric.WithAttributes(labeler.Get()...))
	}

	writer.WriteHeader(statusCode)

	// EventType auto-create feature handling
	if h.EvenTypeHandler != nil {
		h.EvenTypeHandler.AutoCreateEventType(ctx, event, toKReference(broker), broker.GetUID())
	}
}

func toKReference(broker *eventingv1.Broker) *duckv1.KReference {
	kref := &duckv1.KReference{
		Kind:       broker.Kind,
		Namespace:  broker.Namespace,
		Name:       broker.Name,
		APIVersion: broker.APIVersion,
		Address:    broker.Status.Address.Name,
	}
	if kref.Kind == "" {
		kref.Kind = "Broker"
	}
	if kref.APIVersion == "" {
		kref.APIVersion = "eventing.knative.dev/v1"
	}
	return kref
}

func (h *Handler) receive(ctx context.Context, headers http.Header, event *cloudevents.Event, brokerObj *eventingv1.Broker) (int, time.Duration) {
	// Setting the extension as a string as the CloudEvents sdk does not support non-string extensions.
	event.SetExtension(broker.EventArrivalTime, cloudevents.Timestamp{Time: time.Now()})
	if h.Defaulter != nil {
		newEvent := h.Defaulter(ctx, *event)
		event = &newEvent
	}

	if ttl, err := broker.GetTTL(event.Context); err != nil || ttl <= 0 {
		h.Logger.Debug("dropping event based on TTL status.", zap.Int32("TTL", ttl), zap.String("event.id", event.ID()), zap.Error(err))
		return http.StatusBadRequest, kncloudevents.NoDuration
	}

	channelAddress, err := h.getChannelAddress(brokerObj)
	if err != nil {
		h.Logger.Warn("could not get channel address from broker", zap.Error(err))
		return http.StatusInternalServerError, kncloudevents.NoDuration
	}

	ctx = observability.WithMessagingLabels(ctx, channelAddress.URL.String(), "send")

	opts := []kncloudevents.SendOption{
		kncloudevents.WithHeader(headers),
		kncloudevents.WithOIDCAuthentication(&types.NamespacedName{
			Name:      "mt-broker-ingress-oidc",
			Namespace: system.Namespace(),
		}),
	}

	dispatchInfo, err := h.eventDispatcher.SendEvent(ctx, *event, *channelAddress, opts...)
	if err != nil {
		h.Logger.Error("failed to dispatch event", zap.Error(err))
		return http.StatusInternalServerError, kncloudevents.NoDuration
	}

	return dispatchInfo.ResponseCode, dispatchInfo.Duration
}
