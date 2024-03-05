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

	opencensusclient "github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
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
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const (
	defaultMaxIdleConnections        = 1000
	defaultMaxIdleConnectionsPerHost = 1000
)

type Handler struct {
	// Defaults sets default values to incoming events
	Defaulter client.EventDefaulter
	// Reporter reports stats of status code and dispatch time
	Reporter StatsReporter
	// BrokerLister gets broker objects
	BrokerLister eventinglisters.BrokerLister

	EvenTypeHandler *eventtype.EventTypeAutoHandler

	Logger *zap.Logger

	eventDispatcher *kncloudevents.Dispatcher

	tokenVerifier *auth.OIDCTokenVerifier

	withContext func(ctx context.Context) context.Context
}

func NewHandler(logger *zap.Logger, reporter StatsReporter, defaulter client.EventDefaulter, brokerInformer v1.BrokerInformer, tokenVerifier *auth.OIDCTokenVerifier, oidcTokenProvider *auth.OIDCTokenProvider, trustBundleConfigMapLister corev1listers.ConfigMapNamespaceLister, withContext func(ctx context.Context) context.Context) (*Handler, error) {
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
			})
		},
		UpdateFunc: func(_, obj interface{}) {
			broker, ok := obj.(*eventingv1.Broker)
			if !ok || broker == nil || broker.Status.Address == nil {
				return
			}
			kncloudevents.AddOrUpdateAddressableHandler(clientConfig, duckv1.Addressable{
				URL:     broker.Status.Address.URL,
				CACerts: broker.Status.Address.CACerts,
			})
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

	return &Handler{
		Defaulter:       defaulter,
		Reporter:        reporter,
		Logger:          logger,
		BrokerLister:    brokerInformer.Lister(),
		eventDispatcher: kncloudevents.NewDispatcher(clientConfig, oidcTokenProvider),
		tokenVerifier:   tokenVerifier,
		withContext:     withContext,
	}, nil
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
	if err != nil {
		h.Logger.Warn("Failed to retrieve broker", zap.Error(err))
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	features := feature.FromContext(ctx)
	if features.IsOIDCAuthentication() {
		h.Logger.Debug("OIDC authentication is enabled")

		err = h.tokenVerifier.VerifyJWTFromRequest(ctx, request, broker.Status.Address.Audience, writer)
		if err != nil {
			h.Logger.Warn("Error when validating the JWT token in the request", zap.Error(err))
			return
		}

		h.Logger.Debug("Request contained a valid JWT. Continuing...")
	}

	ctx, span := trace.StartSpan(ctx, tracing.BrokerMessagingDestination(brokerNamespacedName))
	defer span.End()

	if span.IsRecordingEvents() {
		span.AddAttributes(
			tracing.MessagingSystemAttribute,
			tracing.MessagingProtocolHTTP,
			tracing.BrokerMessagingDestinationAttribute(brokerNamespacedName),
			tracing.MessagingMessageIDAttribute(event.ID()),
		)
		span.AddAttributes(opencensusclient.EventTraceAttributes(event)...)
	}

	reporterArgs := &ReportArgs{
		ns:        brokerNamespace,
		broker:    brokerName,
		eventType: event.Type(),
	}

	if request.TLS != nil {
		reporterArgs.eventScheme = "https"
	} else {
		reporterArgs.eventScheme = "http"
	}

	statusCode, dispatchTime := h.receive(ctx, utils.PassThroughHeaders(request.Header), event, broker)
	if dispatchTime > kncloudevents.NoDuration {
		_ = h.Reporter.ReportEventDispatchTime(reporterArgs, statusCode, dispatchTime)
	}
	_ = h.Reporter.ReportEventCount(reporterArgs, statusCode)

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
		return http.StatusBadRequest, kncloudevents.NoDuration
	}

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
