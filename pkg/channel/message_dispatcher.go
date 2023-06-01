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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	nethttp "net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/network"
	"knative.dev/pkg/system"

	"knative.dev/eventing/pkg/broker"
	"knative.dev/eventing/pkg/channel/attributes"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

const (
	// noDuration signals that the dispatch step hasn't started
	NoDuration = -1
	NoResponse = -1
)

type MessageDispatcher interface {
	// DispatchMessage dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination duckv1.Addressable, reply *duckv1.Addressable, deadLetter *duckv1.Addressable) (*DispatchExecutionInfo, error)

	// DispatchMessageWithRetries dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination duckv1.Addressable, reply *duckv1.Addressable, deadLetter *duckv1.Addressable, config *kncloudevents.RetryConfig, transformers ...binding.Transformer) (*DispatchExecutionInfo, error)
}

// MessageDispatcherImpl is the 'real' MessageDispatcher used everywhere except unit tests.
var _ MessageDispatcher = &MessageDispatcherImpl{}

// MessageDispatcherImpl dispatches events to a destination over HTTP.
type MessageDispatcherImpl struct {
	supportedSchemes sets.String

	logger *zap.Logger
}

type DispatchExecutionInfo struct {
	Time         time.Duration
	ResponseCode int
	ResponseBody []byte
}

// NewMessageDispatcher creates a new Message dispatcher.
func NewMessageDispatcher(logger *zap.Logger) *MessageDispatcherImpl {
	return &MessageDispatcherImpl{
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

func (d *MessageDispatcherImpl) DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination duckv1.Addressable, reply *duckv1.Addressable, deadLetter *duckv1.Addressable) (*DispatchExecutionInfo, error) {
	return d.DispatchMessageWithRetries(ctx, message, additionalHeaders, destination, reply, deadLetter, nil)
}

func (d *MessageDispatcherImpl) DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination duckv1.Addressable, reply *duckv1.Addressable, deadLetter *duckv1.Addressable, retriesConfig *kncloudevents.RetryConfig, transformers ...binding.Transformer) (*DispatchExecutionInfo, error) {
	// All messages that should be finished at the end of this function
	// are placed in this slice
	var messagesToFinish []binding.Message
	defer func() {
		for _, msg := range messagesToFinish {
			_ = msg.Finish(nil)
		}
	}()

	// sanitize eventual host-only URLs
	destination = *d.sanitizeAddressable(&destination)
	reply = d.sanitizeAddressable(reply)
	deadLetter = d.sanitizeAddressable(deadLetter)

	// If there is a destination, variables response* are filled with the response of the destination
	// Otherwise, they are filled with the original message
	var responseMessage cloudevents.Message
	var responseAdditionalHeaders nethttp.Header
	var dispatchExecutionInfo *DispatchExecutionInfo

	if destination.URL != nil {
		var err error
		// Try to send to destination
		messagesToFinish = append(messagesToFinish, message)

		// Add `Prefer: reply` header no matter if a reply destination is provided. Discussion: https://github.com/knative/eventing/pull/5764
		additionalHeadersForDestination := nethttp.Header{}
		if additionalHeaders != nil {
			additionalHeadersForDestination = additionalHeaders.Clone()
		}
		additionalHeadersForDestination.Set("Prefer", "reply")

		ctx, responseMessage, responseAdditionalHeaders, dispatchExecutionInfo, err = d.executeRequest(ctx, &destination, message, additionalHeadersForDestination, retriesConfig, transformers...)
		if err != nil {
			// If DeadLetter is configured, then send original message with knative error extensions
			if deadLetter != nil {
				dispatchTransformers := d.dispatchExecutionInfoTransformers(destination.URL, dispatchExecutionInfo)
				_, deadLetterResponse, _, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, deadLetter, message, additionalHeaders, retriesConfig, append(transformers, dispatchTransformers)...)
				if deadLetterErr != nil {
					return dispatchExecutionInfo, fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destination.URL, err, deadLetter.URL, deadLetterErr)
				}
				if deadLetterResponse != nil {
					messagesToFinish = append(messagesToFinish, deadLetterResponse)
				}

				return dispatchExecutionInfo, nil
			}
			// No DeadLetter, just fail
			return dispatchExecutionInfo, fmt.Errorf("unable to complete request to %s: %v", destination.URL, err)
		}
	} else {
		// No destination url, try to send to reply if available
		responseMessage = message
		responseAdditionalHeaders = additionalHeaders
	}

	// No response, dispatch completed
	if responseMessage == nil {
		return dispatchExecutionInfo, nil
	}

	messagesToFinish = append(messagesToFinish, responseMessage)

	if reply == nil {
		d.logger.Debug("cannot forward response as reply is empty")
		return dispatchExecutionInfo, nil
	}

	ctx, responseResponseMessage, _, dispatchExecutionInfo, err := d.executeRequest(ctx, reply, responseMessage, responseAdditionalHeaders, retriesConfig, transformers...)
	if err != nil {
		// If DeadLetter is configured, then send original message with knative error extensions
		if deadLetter != nil {
			dispatchTransformers := d.dispatchExecutionInfoTransformers(reply.URL, dispatchExecutionInfo)
			_, deadLetterResponse, _, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, deadLetter, message, responseAdditionalHeaders, retriesConfig, append(transformers, dispatchTransformers)...)
			if deadLetterErr != nil {
				return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s (%v) and failed to send it to the dead letter sink %s (%v)", reply.URL, err, deadLetter.URL, deadLetterErr)
			}
			if deadLetterResponse != nil {
				messagesToFinish = append(messagesToFinish, deadLetterResponse)
			}

			return dispatchExecutionInfo, nil
		}
		// No DeadLetter, just fail
		return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s: %v", reply.URL, err)
	}
	if responseResponseMessage != nil {
		messagesToFinish = append(messagesToFinish, responseResponseMessage)
	}

	return dispatchExecutionInfo, nil
}

func (d *MessageDispatcherImpl) executeRequest(ctx context.Context,
	target *duckv1.Addressable,
	message cloudevents.Message,
	additionalHeaders nethttp.Header,
	configs *kncloudevents.RetryConfig,
	transformers ...binding.Transformer) (context.Context, cloudevents.Message, nethttp.Header, *DispatchExecutionInfo, error) {

	d.logger.Debug("Dispatching event", zap.Any("target", target))

	execInfo := DispatchExecutionInfo{
		Time:         NoDuration,
		ResponseCode: NoResponse,
	}
	ctx, span := trace.StartSpan(ctx, "knative.dev", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req, err := kncloudevents.NewCloudEventRequest(ctx, *target)
	if err != nil {
		return ctx, nil, nil, &execInfo, err
	}

	if span.IsRecordingEvents() {
		transformers = append(transformers, tracing.PopulateSpan(span, target.URL.String()))
	}

	err = kncloudevents.WriteRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders, transformers...)
	if err != nil {
		return ctx, nil, nil, &execInfo, err
	}

	start := time.Now()
	response, err := req.SendWithRetries(configs)
	dispatchTime := time.Since(start)
	if err != nil {
		execInfo.Time = dispatchTime
		execInfo.ResponseCode = nethttp.StatusInternalServerError
		execInfo.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", err.Error()))
		return ctx, nil, nil, &execInfo, err
	}

	if response != nil {
		execInfo.ResponseCode = response.StatusCode
	}
	execInfo.Time = dispatchTime

	body := new(bytes.Buffer)
	_, readErr := body.ReadFrom(response.Body)

	if isFailure(response.StatusCode) {
		// Read response body into execInfo for failures
		if readErr != nil && readErr != io.EOF {
			d.logger.Error("failed to read response body", zap.Error(err))
			execInfo.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", err.Error()))
		} else {
			execInfo.ResponseBody = body.Bytes()
		}
		_ = response.Body.Close()
		// Reject non-successful responses.
		return ctx, nil, nil, &execInfo, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", response.StatusCode)
	}

	var responseMessageBody []byte
	if readErr != nil && readErr != io.EOF {
		d.logger.Error("failed to read response body", zap.Error(err))
		responseMessageBody = []byte(fmt.Sprintf("Failed to read response body: %s", err.Error()))
	} else {
		responseMessageBody = body.Bytes()
	}
	responseMessage := http.NewMessage(response.Header, io.NopCloser(bytes.NewReader(responseMessageBody)))

	if responseMessage.ReadEncoding() == binding.EncodingUnknown {
		_ = response.Body.Close()
		_ = responseMessage.BodyReader.Close()
		d.logger.Debug("Response is a non event, discarding it", zap.Int("status_code", response.StatusCode))
		return ctx, nil, nil, &execInfo, nil
	}
	return ctx, responseMessage, utils.PassThroughHeaders(response.Header), &execInfo, nil
}

func (d *MessageDispatcherImpl) sanitizeAddressable(addressable *duckv1.Addressable) *duckv1.Addressable {
	if addressable == nil {
		return nil
	}

	addressable.URL = d.sanitizeURL(addressable.URL)

	return addressable
}

func (d *MessageDispatcherImpl) sanitizeURL(url *apis.URL) *apis.URL {
	if url == nil {
		return nil
	}

	if d.supportedSchemes.Has(url.Scheme) {
		// Already a URL with a known scheme.
		return url
	}

	return &apis.URL{
		Scheme: "http",
		Host:   url.Host,
		Path:   "/",
	}
}

// dispatchExecutionTransformer returns Transformers based on the specified destination and DispatchExecutionInfo
func (d *MessageDispatcherImpl) dispatchExecutionInfoTransformers(destination *apis.URL, dispatchExecutionInfo *DispatchExecutionInfo) binding.Transformers {
	if destination == nil {
		destination = &apis.URL{}
	}

	httpResponseBody := dispatchExecutionInfo.ResponseBody
	if destination.Host == network.GetServiceHostname("broker-filter", system.Namespace()) {

		var errExtensionInfo broker.ErrExtensionInfo

		err := json.Unmarshal(dispatchExecutionInfo.ResponseBody, &errExtensionInfo)
		if err != nil {
			d.logger.Debug("Unmarshal dispatchExecutionInfo ResponseBody failed", zap.Error(err))
			return nil
		}
		destination = errExtensionInfo.ErrDestination
		httpResponseBody = errExtensionInfo.ErrResponseBody
	}

	destination = d.sanitizeURL(destination)

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
	return statusCode < nethttp.StatusOK /* 200 */ ||
		statusCode >= nethttp.StatusMultipleChoices /* 300 */
}
