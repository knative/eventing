/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"context"

	"go.opencensus.io/trace"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/observability"

	"github.com/cloudevents/sdk-go/v2/protocol"
)

type opencensusObservabilityService struct{}

func (o opencensusObservabilityService) InboundContextDecorators() []func(context.Context, binding.Message) context.Context {
	return []func(context.Context, binding.Message) context.Context{tracePropagatorContextDecorator}
}

func (o opencensusObservabilityService) RecordReceivedMalformedEvent(ctx context.Context, err error) {
	ctx, r := NewReporter(ctx, reportReceive)
	r.Error()
}

func (o opencensusObservabilityService) RecordCallingInvoker(ctx context.Context, event *cloudevents.Event) (context.Context, func(errOrResult error)) {
	ctx, r := NewReporter(ctx, reportReceive)
	return ctx, func(errOrResult error) {
		if protocol.IsACK(errOrResult) {
			r.OK()
		} else {
			r.Error()
		}
	}
}

func (o opencensusObservabilityService) RecordSendingEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error)) {
	ctx, r := NewReporter(ctx, reportSend)
	ctx, span := trace.StartSpan(ctx, observability.ClientSpanName, trace.WithSpanKind(trace.SpanKindClient))
	if span.IsRecordingEvents() {
		span.AddAttributes(EventTraceAttributes(&event)...)
	}

	return ctx, func(errOrResult error) {
		span.End()
		if protocol.IsACK(errOrResult) {
			r.OK()
		} else {
			r.Error()
		}
	}
}

func (o opencensusObservabilityService) RecordRequestEvent(ctx context.Context, event cloudevents.Event) (context.Context, func(errOrResult error, event *cloudevents.Event)) {
	ctx, r := NewReporter(ctx, reportSend)
	ctx, span := trace.StartSpan(ctx, observability.ClientSpanName, trace.WithSpanKind(trace.SpanKindClient))
	if span.IsRecordingEvents() {
		span.AddAttributes(EventTraceAttributes(&event)...)
	}

	return ctx, func(errOrResult error, event *cloudevents.Event) {
		span.End()
		if protocol.IsACK(errOrResult) {
			r.OK()
		} else {
			r.Error()
		}
	}
}

func New() client.ObservabilityService {
	return opencensusObservabilityService{}
}

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
	span := trace.FromContext(messageCtx)
	if span == nil {
		return ctx
	}
	return trace.NewContext(ctx, span)
}
