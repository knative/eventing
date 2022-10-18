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
	"fmt"
	"io"
	nethttp "net/http"
	"net/url"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opencensus.io/trace"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

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
	DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL) (*DispatchExecutionInfo, error)

	// DispatchMessageWithRetries dispatches an event to a destination over HTTP.
	//
	// The destination and reply are URLs.
	DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL, config *kncloudevents.RetryConfig, transformers ...binding.Transformer) (*DispatchExecutionInfo, error)
}

// MessageDispatcherImpl is the 'real' MessageDispatcher used everywhere except unit tests.
var _ MessageDispatcher = &MessageDispatcherImpl{}

// MessageDispatcherImpl dispatches events to a destination over HTTP.
type MessageDispatcherImpl struct {
	sender           *kncloudevents.HTTPMessageSender
	supportedSchemes sets.String

	logger *zap.Logger
}

type DispatchExecutionInfo struct {
	Time         time.Duration
	ResponseCode int
	ResponseBody []byte
}

// NewMessageDispatcher creates a new Message dispatcher based on config.
func NewMessageDispatcher(logger *zap.Logger) *MessageDispatcherImpl {
	sender, err := kncloudevents.NewHTTPMessageSenderWithTarget("")
	if err != nil {
		logger.Fatal("Unable to create cloudevents binding sender", zap.Error(err))
	}
	return NewMessageDispatcherFromSender(logger, sender)
}

// NewMessageDispatcherFromSender creates a new Message dispatcher with specified sender.
func NewMessageDispatcherFromSender(logger *zap.Logger, sender *kncloudevents.HTTPMessageSender) *MessageDispatcherImpl {
	return &MessageDispatcherImpl{
		sender:           sender,
		supportedSchemes: sets.NewString("http", "https"),
		logger:           logger,
	}
}

func (d *MessageDispatcherImpl) DispatchMessage(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL) (*DispatchExecutionInfo, error) {
	return d.DispatchMessageWithRetries(ctx, message, additionalHeaders, destination, reply, deadLetter, nil)
}

func (d *MessageDispatcherImpl) DispatchMessageWithRetries(ctx context.Context, message cloudevents.Message, additionalHeaders nethttp.Header, destination *url.URL, reply *url.URL, deadLetter *url.URL, retriesConfig *kncloudevents.RetryConfig, transformers ...binding.Transformer) (*DispatchExecutionInfo, error) {
	// All messages that should be finished at the end of this function
	// are placed in this slice
	var messagesToFinish []binding.Message
	defer func() {
		for _, msg := range messagesToFinish {
			_ = msg.Finish(nil)
		}
	}()

	// sanitize eventual host-only URLs
	destination = d.sanitizeURL(destination)
	reply = d.sanitizeURL(reply)
	deadLetter = d.sanitizeURL(deadLetter)

	// If there is a destination, variables response* are filled with the response of the destination
	// Otherwise, they are filled with the original message
	var responseMessage cloudevents.Message
	var responseAdditionalHeaders nethttp.Header
	var dispatchExecutionInfo *DispatchExecutionInfo

	if destination != nil {
		var err error
		// Try to send to destination
		messagesToFinish = append(messagesToFinish, message)

		// Add `Prefer: reply` header no matter if a reply destination is provided. Discussion: https://github.com/knative/eventing/pull/5764
		additionalHeadersForDestination := nethttp.Header{}
		if additionalHeaders != nil {
			additionalHeadersForDestination = additionalHeaders.Clone()
		}
		additionalHeadersForDestination.Set("Prefer", "reply")

		ctx, responseMessage, responseAdditionalHeaders, dispatchExecutionInfo, err = d.executeRequest(ctx, destination, message, additionalHeadersForDestination, retriesConfig, transformers...)
		if err != nil {
			// If DeadLetter is configured, then send original message with knative error extensions
			if deadLetter != nil {
				dispatchTransformers := d.dispatchExecutionInfoTransformers(destination, dispatchExecutionInfo)
				_, deadLetterResponse, _, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, deadLetter, message, additionalHeaders, retriesConfig, append(transformers, dispatchTransformers)...)
				if deadLetterErr != nil {
					return dispatchExecutionInfo, fmt.Errorf("unable to complete request to either %s (%v) or %s (%v)", destination, err, deadLetter, deadLetterErr)
				}
				if deadLetterResponse != nil {
					messagesToFinish = append(messagesToFinish, deadLetterResponse)
				}

				return dispatchExecutionInfo, nil
			}
			// No DeadLetter, just fail
			return dispatchExecutionInfo, fmt.Errorf("unable to complete request to %s: %v", destination, err)
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
			dispatchTransformers := d.dispatchExecutionInfoTransformers(reply, dispatchExecutionInfo)
			_, deadLetterResponse, _, dispatchExecutionInfo, deadLetterErr := d.executeRequest(ctx, deadLetter, message, responseAdditionalHeaders, retriesConfig, append(transformers, dispatchTransformers)...)
			if deadLetterErr != nil {
				return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s (%v) and failed to send it to the dead letter sink %s (%v)", reply, err, deadLetter, deadLetterErr)
			}
			if deadLetterResponse != nil {
				messagesToFinish = append(messagesToFinish, deadLetterResponse)
			}

			return dispatchExecutionInfo, nil
		}
		// No DeadLetter, just fail
		return dispatchExecutionInfo, fmt.Errorf("failed to forward reply to %s: %v", reply, err)
	}
	if responseResponseMessage != nil {
		messagesToFinish = append(messagesToFinish, responseResponseMessage)
	}

	return dispatchExecutionInfo, nil
}

func (d *MessageDispatcherImpl) executeRequest(ctx context.Context,
	url *url.URL,
	message cloudevents.Message,
	additionalHeaders nethttp.Header,
	configs *kncloudevents.RetryConfig,
	transformers ...binding.Transformer) (context.Context, cloudevents.Message, nethttp.Header, *DispatchExecutionInfo, error) {

	d.logger.Debug("Dispatching event", zap.String("url", url.String()))

	execInfo := DispatchExecutionInfo{
		Time:         NoDuration,
		ResponseCode: NoResponse,
	}
	ctx, span := trace.StartSpan(ctx, "knative.dev", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	req, err := d.sender.NewCloudEventRequestWithTarget(ctx, url.String())
	if err != nil {
		return ctx, nil, nil, &execInfo, err
	}

	if span.IsRecordingEvents() {
		transformers = append(transformers, tracing.PopulateSpan(span, url.String()))
	}

	err = kncloudevents.WriteHTTPRequestWithAdditionalHeaders(ctx, message, req, additionalHeaders, transformers...)
	if err != nil {
		return ctx, nil, nil, &execInfo, err
	}

	start := time.Now()
	response, err := d.sender.SendWithRetries(req, configs)
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

	body := make([]byte, attributes.KnativeErrorDataExtensionMaxLength)

	if isFailure(response.StatusCode) {
		// Read response body into execInfo for failures
		readLen, err := response.Body.Read(body)
		if err != nil && err != io.EOF {
			d.logger.Error("failed to read response body into DispatchExecutionInfo", zap.Error(err))
			execInfo.ResponseBody = []byte(fmt.Sprintf("dispatch error: %s", err.Error()))
		} else {
			execInfo.ResponseBody = body[:readLen]
		}
		_ = response.Body.Close()
		// Reject non-successful responses.
		return ctx, nil, nil, &execInfo, fmt.Errorf("unexpected HTTP response, expected 2xx, got %d", response.StatusCode)
	}

	var responseMessageBody []byte
	// Read response body into responseMessage for message accepted
	readLen, err := response.Body.Read(body)
	if err != nil && err != io.EOF {
		d.logger.Error("failed to read response body into cloudevents' Message", zap.Error(err))
		responseMessageBody = []byte(fmt.Sprintf("Failed to read response body: %s", err.Error()))
	} else {
		responseMessageBody = body[:readLen]
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

func (d *MessageDispatcherImpl) sanitizeURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	if d.supportedSchemes.Has(u.Scheme) {
		// Already a URL with a known scheme.
		return u
	}
	return &url.URL{
		Scheme: "http",
		Host:   u.Host,
		Path:   "/",
	}
}

// dispatchExecutionTransformer returns Transformers based on the specified destination and DispatchExecutionInfo
func (d *MessageDispatcherImpl) dispatchExecutionInfoTransformers(destination *url.URL, dispatchExecutionInfo *DispatchExecutionInfo) binding.Transformers {
	if destination == nil {
		destination = &url.URL{}
	}
	destination = d.sanitizeURL(destination)
	// Unprintable control characters are not allowed in header values
	// and cause HTTP requests to fail if not removed.
	// https://pkg.go.dev/golang.org/x/net/http/httpguts#ValidHeaderFieldValue
	httpBody := sanitizeHTTPBody(dispatchExecutionInfo.ResponseBody)

	// Encodes response body as base64 for the resulting length.
	bodyLen := len(httpBody)
	encodedLen := base64.StdEncoding.EncodedLen(bodyLen)

	if encodedLen > attributes.KnativeErrorDataExtensionMaxLength {
		encodedLen = attributes.KnativeErrorDataExtensionMaxLength
	}
	encodedBuf := make([]byte, encodedLen)
	base64.StdEncoding.Encode(encodedBuf, []byte(httpBody))

	return attributes.KnativeErrorTransformers(*destination, dispatchExecutionInfo.ResponseCode, string(encodedBuf[:encodedLen]))
}

func sanitizeHTTPBody(body []byte) string {
	if !hasControlChars(body) {
		return string(body)
	}

	sanitizedResponse := make([]byte, 0, len(body))
	for _, v := range body {
		if !isControl(v) {
			sanitizedResponse = append(sanitizedResponse, v)
		}
	}
	return string(sanitizedResponse)
}

func hasControlChars(data []byte) bool {
	for _, v := range data {
		if isControl(v) {
			return true
		}
	}
	return false
}

func isControl(c byte) bool {
	// US ASCII codes range for printable graphic characters and a space.
	// http://www.columbia.edu/kermit/ascii.html
	const asciiUnitSeparator = 31
	const asciiRubout = 127

	return int(c) < asciiUnitSeparator || int(c) > asciiRubout
}

// isFailure returns true if the status code is not a successful HTTP status.
func isFailure(statusCode int) bool {
	return statusCode < nethttp.StatusOK /* 200 */ ||
		statusCode >= nethttp.StatusMultipleChoices /* 300 */
}
