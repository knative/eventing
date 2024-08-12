/*
Copyright 2023 The Knative Authors

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

package kncloudevents

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	"github.com/cloudevents/sdk-go/v2/event"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/hashicorp/go-retryablehttp"
	"go.opencensus.io/trace"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"

	eventingapis "knative.dev/eventing/pkg/apis"
	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/eventtype"
	"knative.dev/eventing/pkg/utils"

	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/kncloudevents/attributes"
	"knative.dev/eventing/pkg/tracing"
)

const (
	// noDuration signals that the dispatch step hasn't started
	NoDuration = -1
	NoResponse = -1
)

type DispatchInfo struct {
	Duration       time.Duration
	ResponseCode   int
	ResponseHeader http.Header
	ResponseBody   []byte
	Scheme         string
}

type SendOption func(*senderConfig) error

func WithEventFormat(format *v1.FormatType) SendOption {
	return func(sc *senderConfig) error {
		sc.eventFormat = format
		return nil
	}
}

func WithReply(reply *duckv1.Addressable) SendOption {
	return func(sc *senderConfig) error {
		sc.reply = reply

		return nil
	}
}

func WithDeadLetterSink(dls *duckv1.Addressable) SendOption {
	return func(sc *senderConfig) error {
		sc.deadLetterSink = dls

		return nil
	}
}

func WithRetryConfig(retryConfig *RetryConfig) SendOption {
	return func(sc *senderConfig) error {
		sc.retryConfig = retryConfig

		return nil
	}
}

func WithHeader(header http.Header) SendOption {
	return func(sc *senderConfig) error {
		sc.additionalHeaders = header

		return nil
	}
}

func WithTransformers(transformers ...binding.Transformer) SendOption {
	return func(sc *senderConfig) error {
		sc.transformers = transformers

		return nil
	}
}

func WithOIDCAuthentication(serviceAccount *types.NamespacedName) SendOption {
	return func(sc *senderConfig) error {
		if serviceAccount != nil && serviceAccount.Name != "" && serviceAccount.Namespace != "" {
			sc.oidcServiceAccount = serviceAccount
			return nil
		} else {
			return fmt.Errorf("service account name and namespace for OIDC authentication must not be empty")
		}
	}
}

func WithEventTypeAutoHandler(handler *eventtype.EventTypeAutoHandler, ref *duckv1.KReference, ownerUID types.UID) SendOption {
	return func(sc *senderConfig) error {
		if handler != nil && (ref == nil || ownerUID == types.UID("")) {
			return fmt.Errorf("addressable and ownerUID must be provided if using the eventtype auto handler")
		}
		sc.eventTypeAutoHandler = handler
		sc.eventTypeRef = ref
		sc.eventTypeOnwerUID = ownerUID

		return nil
	}
}

type senderConfig struct {
	reply                *duckv1.Addressable
	deadLetterSink       *duckv1.Addressable
	additionalHeaders    http.Header
	retryConfig          *RetryConfig
	transformers         binding.Transformers
	oidcServiceAccount   *types.NamespacedName
	eventTypeAutoHandler *eventtype.EventTypeAutoHandler
	eventTypeRef         *duckv1.KReference
	eventTypeOnwerUID    types.UID
	eventFormat          *v1.FormatType
}

type Dispatcher struct {
	oidcTokenProvider *auth.OIDCTokenProvider
	clientConfig      eventingtls.ClientConfig
}

func NewDispatcher(clientConfig eventingtls.ClientConfig, oidcTokenProvider *auth.OIDCTokenProvider) *Dispatcher {
	return &Dispatcher{
		clientConfig:      clientConfig,
		oidcTokenProvider: oidcTokenProvider,
	}
}

// SendEvent sends the given event to the given destination.
func (d *Dispatcher) SendEvent(ctx context.Context, event event.Event, destination duckv1.Addressable, options ...SendOption) (*DispatchInfo, error) {
	// clone the event since:
	// - we mutate the event and the callers might not expect this
	// - it might produce data races if the caller is trying to read the event in different go routines
	c := event.Clone()
	message := binding.ToMessage(&c)

	return d.SendMessage(ctx, message, destination, options...)
}

// SendMessage sends the given message to the given destination.
// SendMessage is kept for compatibility and SendEvent should be used whenever possible.
func (d *Dispatcher) SendMessage(ctx context.Context, message binding.Message, destination duckv1.Addressable, options ...SendOption) (*DispatchInfo, error) {
	config := &senderConfig{
		additionalHeaders: make(http.Header),
	}

	// apply options
	for _, opt := range options {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("could not apply option: %w", err)
		}
	}

	return d.send(ctx, message, destination, config)
}

func (d *Dispatcher) send(ctx context.Context, message binding.Message, destination duckv1.Addressable, config *senderConfig) (*DispatchInfo, error) {
	dispatchExecutionInfo := &DispatchInfo{}

	// All messages that should be finished at the end of this function
	// are placed in this slice
	messagesToFinish := []binding.Message{message}
	defer func() {
		for _, msg := range messagesToFinish {
			_ = msg.Finish(nil)
		}
	}()

	if destination.URL == nil {
		return dispatchExecutionInfo, fmt.Errorf("can not dispatch message to nil destination.URL")
	}

	// sanitize eventual host-only URLs
	destination = *sanitizeAddressable(&destination)
	config.reply = sanitizeAddressable(config.reply)
	config.deadLetterSink = sanitizeAddressable(config.deadLetterSink)

	// send to destination

	// Add `Prefer: reply` header no matter if a reply destination is provided. Discussion: https://github.com/knative/eventing/pull/5764
	additionalHeadersForDestination := http.Header{}
	if config.additionalHeaders != nil {
		additionalHeadersForDestination = config.additionalHeaders.Clone()
	}
	additionalHeadersForDestination.Set("Prefer", "reply")

	// Handle the event format option
	if config.eventFormat != nil {
		switch *config.eventFormat {
		case v1.DeliveryFormatBinary:
			ctx = binding.WithForceBinary(ctx)
		case v1.DeliveryFormatJson:
			ctx = binding.WithForceStructured(ctx)
		}
	}

	ctx, responseMessage, dispatchExecutionInfo, err := d.executeRequest(ctx, destination, message, additionalHeadersForDestination, config.retryConfig, config.oidcServiceAccount, config.transformers)
	if err != nil {
		// If DeadLetter is configured, then send original message with knative error extensions
		if config.deadLetterSink != nil {
			dispatchTransformers := dispatchExecutionInfoTransformers(destination.URL, dispatchExecutionInfo)
			_, deadLetterResponse, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, *config.deadLetterSink, message, config.additionalHeaders, config.retryConfig, config.oidcServiceAccount, append(config.transformers, dispatchTransformers))
			if deadLetterErr != nil {
				return dispatchExecutionInfo, fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destination.URL, err, config.deadLetterSink.URL, deadLetterErr)
			}
			if deadLetterResponse != nil {
				messagesToFinish = append(messagesToFinish, deadLetterResponse)
			}

			return dispatchExecutionInfo, nil
		}
		// No DeadLetter, just fail
		return dispatchExecutionInfo, fmt.Errorf("unable to complete request to %s: %w", destination.URL, err)
	}

	responseAdditionalHeaders := utils.PassThroughHeaders(dispatchExecutionInfo.ResponseHeader)

	if config.additionalHeaders.Get(eventingapis.KnNamespaceHeader) != "" {
		if responseAdditionalHeaders == nil {
			responseAdditionalHeaders = make(http.Header)
		}
		responseAdditionalHeaders.Set(eventingapis.KnNamespaceHeader, config.additionalHeaders.Get(eventingapis.KnNamespaceHeader))
	}

	if responseMessage == nil {
		// No response, dispatch completed
		return dispatchExecutionInfo, nil
	}

	messagesToFinish = append(messagesToFinish, responseMessage)

	if config.eventTypeAutoHandler != nil {
		// messages can only be read once, so we need to make a copy of it
		responseMessage, err = buffering.CopyMessage(ctx, responseMessage)
		if err == nil {
			d.handleAutocreate(ctx, responseMessage, config)
		}
	}

	if config.reply == nil {
		return dispatchExecutionInfo, nil
	}

	// send reply
	ctx, responseResponseMessage, dispatchExecutionInfo, err := d.executeRequest(ctx, *config.reply, responseMessage, responseAdditionalHeaders, config.retryConfig, config.oidcServiceAccount, config.transformers)
	if err != nil {
		// If DeadLetter is configured, then send original message with knative error extensions
		if config.deadLetterSink != nil {
			dispatchTransformers := dispatchExecutionInfoTransformers(config.reply.URL, dispatchExecutionInfo)
			_, deadLetterResponse, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, *config.deadLetterSink, message, responseAdditionalHeaders, config.retryConfig, config.oidcServiceAccount, append(config.transformers, dispatchTransformers))
			if deadLetterErr != nil {
				return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s (%v) and failed to send it to the dead letter sink %s (%v)", config.reply.URL, err, config.deadLetterSink.URL, deadLetterErr)
			}
			if deadLetterResponse != nil {
				messagesToFinish = append(messagesToFinish, deadLetterResponse)
			}

			return dispatchExecutionInfo, nil
		}
		// No DeadLetter, just fail
		return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s: %w", config.reply.URL, err)
	}
	if responseResponseMessage != nil {
		messagesToFinish = append(messagesToFinish, responseResponseMessage)
	}

	return dispatchExecutionInfo, nil
}

func (d *Dispatcher) executeRequest(ctx context.Context, target duckv1.Addressable, message cloudevents.Message, additionalHeaders http.Header, retryConfig *RetryConfig, oidcServiceAccount *types.NamespacedName, transformers ...binding.Transformer) (context.Context, cloudevents.Message, *DispatchInfo, error) {
	var scheme string
	if target.URL != nil {
		scheme = target.URL.Scheme
	} else {
		// assume that the scheme is http by default
		scheme = "http"
	}
	dispatchInfo := DispatchInfo{
		Duration:       NoDuration,
		ResponseCode:   NoResponse,
		ResponseHeader: make(http.Header),
		Scheme:         scheme,
	}

	ctx, span := trace.StartSpan(ctx, "knative.dev", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if span.IsRecordingEvents() {
		transformers = append(transformers, tracing.PopulateSpan(span, target.URL.String()))
	}

	req, err := d.createRequest(ctx, message, target, additionalHeaders, oidcServiceAccount, transformers...)
	if err != nil {
		return ctx, nil, &dispatchInfo, fmt.Errorf("failed to create request: %w", err)
	}

	client, err := newClient(d.clientConfig, target)
	if err != nil {
		return ctx, nil, &dispatchInfo, fmt.Errorf("failed to create http client: %w", err)
	}

	start := time.Now()
	response, err := client.DoWithRetries(req, retryConfig)
	dispatchInfo.Duration = time.Since(start)
	if err != nil {
		dispatchInfo.ResponseCode = http.StatusInternalServerError
		dispatchInfo.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", err.Error()))

		return ctx, nil, &dispatchInfo, err
	}

	dispatchInfo.ResponseCode = response.StatusCode
	dispatchInfo.ResponseHeader = response.Header

	body := new(bytes.Buffer)
	_, err = body.ReadFrom(response.Body)

	if isFailure(response.StatusCode) {
		// Read response body into dispatchInfo for failures
		if err != nil && err != io.EOF {
			dispatchInfo.ResponseBody = []byte(fmt.Sprintf("dispatch resulted in status \"%s\". Could not read response body: error: %s", response.Status, err.Error()))
		} else {
			dispatchInfo.ResponseBody = body.Bytes()
		}
		response.Body.Close()

		// Reject non-successful responses.
		return ctx, nil, &dispatchInfo, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", response.StatusCode)
	}

	var responseMessageBody []byte
	if err != nil && err != io.EOF {
		responseMessageBody = []byte(fmt.Sprintf("Failed to read response body: %s", err.Error()))
		dispatchInfo.ResponseCode = http.StatusInternalServerError
	} else {
		responseMessageBody = body.Bytes()
		dispatchInfo.ResponseBody = responseMessageBody
	}
	responseMessage := cehttp.NewMessage(response.Header, io.NopCloser(bytes.NewReader(responseMessageBody)))

	if responseMessage.ReadEncoding() == binding.EncodingUnknown {
		// Response is a non event, discard it
		response.Body.Close()
		responseMessage.BodyReader.Close()
		return ctx, nil, &dispatchInfo, nil
	}

	return ctx, responseMessage, &dispatchInfo, nil
}

func (d *Dispatcher) handleAutocreate(ctx context.Context, msg binding.Message, config *senderConfig) {
	responseEvent, err := binding.ToEvent(ctx, msg)
	if err != nil {
		return
	}

	config.eventTypeAutoHandler.AutoCreateEventType(ctx, responseEvent, config.eventTypeRef, config.eventTypeOnwerUID)
}

func (d *Dispatcher) createRequest(ctx context.Context, message binding.Message, target duckv1.Addressable, additionalHeaders http.Header, oidcServiceAccount *types.NamespacedName, transformers ...binding.Transformer) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, "POST", target.URL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("could not create http request: %w", err)
	}

	if err := cehttp.WriteRequest(ctx, message, request, transformers...); err != nil {
		return nil, fmt.Errorf("could not write message to request: %w", err)
	}

	for key, val := range additionalHeaders {
		request.Header[key] = val
	}

	if oidcServiceAccount != nil {
		if target.Audience != nil && *target.Audience != "" {
			jwt, err := d.oidcTokenProvider.GetJWT(*oidcServiceAccount, *target.Audience)
			if err != nil {
				return nil, fmt.Errorf("could not get JWT: %w", err)
			}
			request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwt))
		}
	}

	return request, nil
}

// client is a wrapper around the http.Client, which provides methods for retries
type client struct {
	http.Client
}

func newClient(cfg eventingtls.ClientConfig, target duckv1.Addressable) (*client, error) {
	c, err := getClientForAddressable(cfg, target)
	if err != nil {
		return nil, fmt.Errorf("failed to get http client for addressable: %w", err)
	}

	return &client{
		Client: *c,
	}, nil
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	return c.Client.Do(req)
}

func (c *client) DoWithRetries(req *http.Request, retryConfig *RetryConfig) (*http.Response, error) {
	if retryConfig == nil {
		return c.Do(req)
	}

	client := c.Client
	if retryConfig.RequestTimeout != 0 {
		client = http.Client{
			Transport:     client.Transport,
			CheckRedirect: client.CheckRedirect,
			Jar:           client.Jar,
			Timeout:       retryConfig.RequestTimeout,
		}
	}

	retryableClient := retryablehttp.Client{
		HTTPClient:   &client,
		RetryWaitMin: defaultRetryWaitMin,
		RetryWaitMax: defaultRetryWaitMax,
		RetryMax:     retryConfig.RetryMax,
		CheckRetry:   retryablehttp.CheckRetry(retryConfig.CheckRetry),
		Backoff:      generateBackoffFn(retryConfig),
		ErrorHandler: func(resp *http.Response, err error, numTries int) (*http.Response, error) {
			return resp, err
		},
	}

	retryableReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	return retryableClient.Do(retryableReq)
}

// dispatchExecutionTransformer returns Transformers based on the specified destination and DispatchExecutionInfo
func dispatchExecutionInfoTransformers(destination *apis.URL, dispatchExecutionInfo *DispatchInfo) binding.Transformers {
	if destination == nil {
		destination = &apis.URL{}
	}

	httpResponseBody := dispatchExecutionInfo.ResponseBody
	if destination.Host == network.GetServiceHostname("broker-filter", system.Namespace()) {

		var errExtensionInfo broker.ErrExtensionInfo

		err := json.Unmarshal(dispatchExecutionInfo.ResponseBody, &errExtensionInfo)
		if err != nil {
			return nil
		}
		destination = errExtensionInfo.ErrDestination
		httpResponseBody = errExtensionInfo.ErrResponseBody
	}

	destination = sanitizeURL(destination)

	// Encodes response body as base64 for the resulting length.
	bodyLen := len(httpResponseBody)
	encodedLen := base64.StdEncoding.EncodedLen(bodyLen)
	if encodedLen > attributes.KnativeErrorDataExtensionMaxLength {
		encodedLen = attributes.KnativeErrorDataExtensionMaxLength
	}
	encodedBuf := make([]byte, encodedLen)
	base64.StdEncoding.Encode(encodedBuf, httpResponseBody)

	return attributes.KnativeErrorTransformers(*destination.URL(), dispatchExecutionInfo.ResponseCode, string(encodedBuf[:encodedLen]))
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < http.StatusOK /* 200 */ ||
		statusCode >= http.StatusMultipleChoices /* 300 */
}

func sanitizeAddressable(addressable *duckv1.Addressable) *duckv1.Addressable {
	if addressable == nil {
		return nil
	}

	addressable.URL = sanitizeURL(addressable.URL)

	return addressable
}

func sanitizeURL(url *apis.URL) *apis.URL {
	if url == nil {
		return nil
	}

	if url.Scheme == "http" || url.Scheme == "https" {
		// Already a URL with a known scheme.
		return url
	}

	return &apis.URL{
		Scheme: "http",
		Host:   url.Host,
		Path:   "/",
	}
}
