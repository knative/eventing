/*
Copyright 2020 The Knative Authors

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

package channel

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol/http"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/network"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/observability"
	"knative.dev/eventing/pkg/utils"
)

const (
	ScopeName = "knative.dev/eventing/pkg/channel"
)

// UnknownChannelError represents the error when an event is received by a channel dispatcher for a
// channel that does not exist.
type UnknownChannelError struct {
	Channel ChannelReference
}

func (e *UnknownChannelError) Error() string {
	return fmt.Sprint("unknown channel: ", e.Channel)
}

// UnknownHostError represents the error when a ResolveChannelFromHostHeader func cannot resolve an host
type UnknownHostError string

func (e UnknownHostError) Error() string {
	return "cannot map host to channel: " + string(e)
}

type BadRequestError string

func (e BadRequestError) Error() string {
	return "malformed request: " + string(e)
}

// EventReceiver starts a server to receive new events for the channel dispatcher. The new
// event is emitted via the receiver function.
type EventReceiver struct {
	httpBindingsReceiver *kncloudevents.HTTPEventReceiver
	receiverFunc         EventReceiverFunc
	logger               *zap.Logger
	hostToChannelFunc    ResolveChannelFromHostFunc
	pathToChannelFunc    ResolveChannelFromPathFunc
	tokenVerifier        *auth.Verifier
	audience             string
	getPoliciesForFunc   GetPoliciesForFunc
	withContext          func(context.Context) context.Context
	meterProvider        metric.MeterProvider
	traceProvider        trace.TracerProvider
}

// EventReceiverFunc is the function to be called for handling the event.
type EventReceiverFunc func(context.Context, ChannelReference, event.Event, nethttp.Header) error

// ReceiverOptions provides functional options to EventReceiver function.
type EventReceiverOptions func(*EventReceiver) error

// ResolveChannelFromHostFunc function enables EventReceiver to get the Channel Reference from incoming request HostHeader
// before calling receiverFunc.
// Returns UnknownHostError if the channel is not found, otherwise returns a generic error.
type ResolveChannelFromHostFunc func(string) (ChannelReference, error)

// ResolveChannelFromHostHeader is a ReceiverOption for NewEventReceiver which enables the caller to overwrite the
// default behaviour defined by ParseChannelFromHost function.
func ResolveChannelFromHostHeader(hostToChannelFunc ResolveChannelFromHostFunc) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.hostToChannelFunc = hostToChannelFunc
		return nil
	}
}

// ResolveChannelFromPathFunc function enables EventReceiver to get the Channel Reference from incoming request's path
// before calling receiverFunc.
type ResolveChannelFromPathFunc func(string) (ChannelReference, error)

// ResolveChannelFromPath is a ReceiverOption for NewEventReceiver which enables the caller to overwrite the
// default behaviour defined by ParseChannelFromPath function.
func ResolveChannelFromPath(PathToChannelFunc ResolveChannelFromPathFunc) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.pathToChannelFunc = PathToChannelFunc
		return nil
	}
}

// GetPoliciesForFunc function enables the EventReceiver to get the Channels AppliedEventPoliciesStatus
type GetPoliciesForFunc func(channel ChannelReference) ([]duckv1.AppliedEventPolicyRef, error)

func ReceiverWithGetPoliciesForFunc(fn GetPoliciesForFunc) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.getPoliciesForFunc = fn
		return nil
	}
}

func OIDCTokenVerification(tokenVerifier *auth.Verifier, audience string) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.tokenVerifier = tokenVerifier
		r.audience = audience
		return nil
	}
}

func ReceiverWithContextFunc(fn func(context.Context) context.Context) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.withContext = fn
		return nil
	}
}

func MeterProvider(meterProvider metric.MeterProvider) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.meterProvider = meterProvider
		return nil
	}
}

func TraceProvider(traceProvider trace.TracerProvider) EventReceiverOptions {
	return func(r *EventReceiver) error {
		r.traceProvider = traceProvider
		return nil
	}
}

// NewEventReceiver creates an event receiver passing new events to the
// receiverFunc.
func NewEventReceiver(receiverFunc EventReceiverFunc, logger *zap.Logger, opts ...EventReceiverOptions) (*EventReceiver, error) {
	bindingsReceiver := kncloudevents.NewHTTPEventReceiver(8080)
	receiver := &EventReceiver{
		httpBindingsReceiver: bindingsReceiver,
		receiverFunc:         receiverFunc,
		hostToChannelFunc:    ResolveChannelFromHostFunc(ParseChannelFromHost),
		logger:               logger,
	}
	for _, opt := range opts {
		if err := opt(receiver); err != nil {
			return nil, err
		}
	}

	if receiver.traceProvider == nil {
		receiver.traceProvider = otel.GetTracerProvider()
	}
	if receiver.meterProvider == nil {
		receiver.meterProvider = otel.GetMeterProvider()
	}

	return receiver, nil
}

// Start begins to receive events for the receiver.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
func (r *EventReceiver) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	otelOpts := []otelhttp.Option{}
	if r.meterProvider != nil {
		otelOpts = append(otelOpts, otelhttp.WithMeterProvider(r.meterProvider))
	}
	if r.traceProvider != nil {
		otelOpts = append(otelOpts, otelhttp.WithTracerProvider(r.traceProvider))
	}

	go func() {
		errCh <- r.httpBindingsReceiver.StartListen(ctx, r, otelOpts...)
	}()

	// Stop either if the receiver stops (sending to errCh) or if the context Done channel is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		break
	}

	// Done channel has been closed, we need to gracefully shutdown r.ceClient. The cancel() method will start its
	// shutdown, if it hasn't finished in a reasonable amount of Time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(network.DefaultDrainTimeout):
		return errors.New("timeout shutting down http bindings receiver")
	}
}

func (r *EventReceiver) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	ctx := request.Context()

	if r.withContext != nil {
		ctx = r.withContext(ctx)
	}

	response.Header().Set("Allow", "POST, OPTIONS")
	if request.Method == nethttp.MethodOptions {
		response.Header().Set("WebHook-Allowed-Origin", "*") // Accept from any Origin:
		response.Header().Set("WebHook-Allowed-Rate", "*")   // Unlimited requests/minute
		response.WriteHeader(nethttp.StatusOK)
		return
	}
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	// The response status codes:
	//   202 - the event was sent to subscribers
	//   400 - the request was malformed
	//   404 - the request was for an unknown channel
	//   500 - an error occurred processing the request
	var channel ChannelReference
	var err error

	// prefer using pathToChannelFunc if available
	if r.pathToChannelFunc != nil {
		channel, err = r.pathToChannelFunc(request.URL.Path)
	} else {
		if request.URL.Path != "/" {
			response.WriteHeader(nethttp.StatusBadRequest)
			return
		}
		channel, err = r.hostToChannelFunc(request.Host)
	}

	if err != nil {
		switch err.(type) {
		case UnknownHostError:
			response.WriteHeader(nethttp.StatusNotFound)
		case BadRequestError:
			response.WriteHeader(nethttp.StatusBadRequest)
		default:
			response.WriteHeader(nethttp.StatusInternalServerError)
		}

		r.logger.Info("Could not extract channel", zap.Error(err))
		return
	}
	r.logger.Debug("Request mapped to channel", zap.String("channel", channel.String()))

	ctx = observability.WithChannelLabels(ctx, types.NamespacedName{Name: channel.Name, Namespace: channel.Namespace})
	ctx = observability.WithRequestLabels(ctx, request)

	// copy the request, as we need access to the body (in case of a structured event) for the auth checks too
	reqCopy, err := utils.CopyRequest(request)
	if err != nil {
		r.logger.Error("Failed to copy request", zap.Error(err))
		response.WriteHeader(nethttp.StatusInternalServerError)
		return
	}

	event, err := http.NewEventFromHTTPRequest(request)
	if err != nil {
		r.logger.Warn("failed to extract event from request", zap.Error(err))
		response.WriteHeader(nethttp.StatusBadRequest)
		return
	}

	ctx = observability.WithMinimalEventLabels(ctx, event)

	// run validation for the extracted event
	if err := event.Validate(); err != nil {
		r.logger.Warn("failed to validate extracted event", zap.Error(err))
		response.WriteHeader(nethttp.StatusBadRequest)
		return
	}

	// Here we do the OIDC audience verification
	features := feature.FromContext(ctx)
	if features.IsOIDCAuthentication() {
		r.logger.Debug("OIDC authentication is enabled")

		if r.getPoliciesForFunc == nil {
			r.logger.Error("getPoliciesForFunc() callback not set. Can't get applying event policies of channel")
			response.WriteHeader(nethttp.StatusInternalServerError)
			return
		}

		applyingEventPolicies, err := r.getPoliciesForFunc(channel)
		if err != nil {
			r.logger.Error("could not get applying event policies of channel", zap.Error(err), zap.String("channel", channel.String()))
			response.WriteHeader(nethttp.StatusInternalServerError)
			return
		}

		err = r.tokenVerifier.VerifyRequest(ctx, features, &r.audience, channel.Namespace, applyingEventPolicies, reqCopy, response)
		if err != nil {
			r.logger.Warn("could not verify authn and authz of request", zap.Error(err))
			return
		}
		r.logger.Debug("Request contained a valid and authorized JWT. Continuing...")
	}

	err = r.receiverFunc(request.Context(), channel, *event, utils.PassThroughHeaders(request.Header))
	if err != nil {
		if _, ok := err.(*UnknownChannelError); ok {
			response.WriteHeader(nethttp.StatusNotFound)
		} else {
			r.logger.Info("Error in receiver", zap.Error(err))
			response.WriteHeader(nethttp.StatusInternalServerError)
		}
		return
	}
	response.WriteHeader(nethttp.StatusAccepted)
}

var _ nethttp.Handler = (*EventReceiver)(nil)
