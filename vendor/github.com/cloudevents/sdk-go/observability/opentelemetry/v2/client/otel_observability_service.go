/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"context"
	"runtime"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/observability"
	"github.com/cloudevents/sdk-go/v2/protocol"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

// OTelObservabilityService implements the ObservabilityService interface from cloudevents
type OTelObservabilityService struct {
	tracer               trace.Tracer
	spanAttributesGetter func(cloudevents.Event) []attribute.KeyValue
	spanNameFormatter    func(cloudevents.Event) string
}

// NewOTelObservabilityService returns an OpenTelemetry-enabled observability service
func NewOTelObservabilityService(opts ...OTelObservabilityServiceOption) *OTelObservabilityService {
	tracerProvider := otel.GetTracerProvider()

	o := &OTelObservabilityService{
		tracer: tracerProvider.Tracer(
			instrumentationName,
			// TODO: Can we have the package version here?
			// trace.WithInstrumentationVersion("1.0.0"),
		),
		spanNameFormatter: defaultSpanNameFormatter,
	}

	// apply passed options
	for _, opt := range opts {
		opt(o)
	}

	return o
}

// InboundContextDecorators returns a decorator function that allows enriching the context with the incoming parent trace.
// This method gets invoked automatically by passing the option 'WithObservabilityService' when creating the cloudevents HTTP client.
func (o OTelObservabilityService) InboundContextDecorators() []func(context.Context, binding.Message) context.Context {
	return []func(context.Context, binding.Message) context.Context{tracePropagatorContextDecorator}
}

// RecordReceivedMalformedEvent records the error from a malformed event in the span.
func (o OTelObservabilityService) RecordReceivedMalformedEvent(ctx context.Context, err error) {
	spanName := observability.ClientSpanName + ".malformed receive"
	_, span := o.tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attribute.String(string(semconv.CodeFunctionKey), getFuncName())))

	recordSpanError(span, err)
	span.End()
}

// RecordCallingInvoker starts a new span before calling the invoker upon a received event.
// In case the operation fails, the error is recorded and the span is marked as failed.
func (o OTelObservabilityService) RecordCallingInvoker(ctx context.Context, event *cloudevents.Event) (context.Context, func(errOrResult error)) {
	spanName := o.getSpanName(event, "process")
	ctx, span := o.tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(GetDefaultSpanAttributes(event, getFuncName())...))

	if span.IsRecording() && o.spanAttributesGetter != nil {
		span.SetAttributes(o.spanAttributesGetter(*event)...)
	}

	return ctx, func(errOrResult error) {
		recordSpanError(span, errOrResult)
		span.End()
	}
}

// RecordSendingEvent starts a new span before sending the event.
// In case the operation fails, the error is recorded and the span is marked as failed.
func (o OTelObservabilityService) RecordSendingEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error)) {
	spanName := o.getSpanName(&event, "send")

	ctx, span := o.tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(GetDefaultSpanAttributes(&event, getFuncName())...))

	if span.IsRecording() && o.spanAttributesGetter != nil {
		span.SetAttributes(o.spanAttributesGetter(event)...)
	}

	return ctx, func(errOrResult error) {
		recordSpanError(span, errOrResult)
		span.End()
	}
}

// RecordRequestEvent starts a new span before transmitting the given request.
// In case the operation fails, the error is recorded and the span is marked as failed.
func (o OTelObservabilityService) RecordRequestEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error, event *cloudevents.Event)) {
	spanName := o.getSpanName(&event, "send")

	ctx, span := o.tracer.Start(
		ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(GetDefaultSpanAttributes(&event, getFuncName())...))

	if span.IsRecording() && o.spanAttributesGetter != nil {
		span.SetAttributes(o.spanAttributesGetter(event)...)
	}

	return ctx, func(errOrResult error, event *cloudevents.Event) {
		recordSpanError(span, errOrResult)
		span.End()
	}
}

// GetDefaultSpanAttributes returns the attributes that are always added to the spans
// created by the OTelObservabilityService.
func GetDefaultSpanAttributes(e *cloudevents.Event, method string) []attribute.KeyValue {
	attr := []attribute.KeyValue{
		attribute.String(string(semconv.CodeFunctionKey), method),
		attribute.String(observability.SpecversionAttr, e.SpecVersion()),
		attribute.String(observability.IdAttr, e.ID()),
		attribute.String(observability.TypeAttr, e.Type()),
		attribute.String(observability.SourceAttr, e.Source()),
	}
	if sub := e.Subject(); sub != "" {
		attr = append(attr, attribute.String(observability.SubjectAttr, sub))
	}
	if dct := e.DataContentType(); dct != "" {
		attr = append(attr, attribute.String(observability.DatacontenttypeAttr, dct))
	}
	return attr
}

// Extracts the traceparent from the msg and enriches the context to enable propagation
func tracePropagatorContextDecorator(ctx context.Context, msg binding.Message) context.Context {
	var messageCtx context.Context
	if mctx, ok := msg.(binding.MessageContext); ok {
		messageCtx = mctx.Context()
	} else if mctx, ok := binding.UnwrapMessage(msg).(binding.MessageContext); ok {
		messageCtx = mctx.Context()
	}

	if messageCtx == nil {
		return ctx
	}
	span := trace.SpanFromContext(messageCtx)
	if span == nil {
		return ctx
	}
	return trace.ContextWithSpan(ctx, span)
}

func recordSpanError(span trace.Span, errOrResult error) {
	if protocol.IsACK(errOrResult) || !span.IsRecording() {
		return
	}

	var httpResult *cehttp.Result
	if cloudevents.ResultAs(errOrResult, &httpResult) {
		span.RecordError(httpResult)
		if httpResult.StatusCode > 0 {
			code, _ := semconv.SpanStatusFromHTTPStatusCode(httpResult.StatusCode)
			span.SetStatus(code, httpResult.Error())
		}
	} else {
		span.RecordError(errOrResult)
	}
}

// getSpanName Returns the name of the span.
//
// When no spanNameFormatter is present in OTelObservabilityService,
// the default name will be "cloudevents.client.<eventtype> prefix" e.g. cloudevents.client.get.customers send.
//
// The prefix is always added at the end of the span name. This follows the semantic conventions for
// messasing systems as defined in https://github.com/open-telemetry/opentelemetry-specification/blob/v1.6.1/specification/trace/semantic_conventions/messaging.md#operation-names
func (o OTelObservabilityService) getSpanName(e *cloudevents.Event, suffix string) string {
	name := o.spanNameFormatter(*e)

	// make sure the span name ends with the suffix from the semantic conventions (receive, send, process)
	if !strings.HasSuffix(name, suffix) {
		return name + " " + suffix
	}

	return name
}

func getFuncName() string {
	pc := make([]uintptr, 1)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()

	// frame.Function should be github.com/cloudevents/sdk-go/observability/opentelemetry/v2/client.OTelObservabilityService.Func
	parts := strings.Split(frame.Function, ".")

	// we are interested in the function name
	if len(parts) != 4 {
		return ""
	}
	return parts[3]
}
