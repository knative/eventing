/*
Copyright 2022 The Knative Authors

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

package client

import (
	"context"

	obsclient "github.com/cloudevents/sdk-go/observability/opencensus/v2/client"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	ceobs "github.com/cloudevents/sdk-go/v2/observability"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"go.opencensus.io/trace"

	"knative.dev/eventing/pkg/observability"
)

func New() knativeObservabilityService {
	return knativeObservabilityService{
		obsclient.New(),
	}
}

type knativeObservabilityService struct {
	base client.ObservabilityService
}

func (k knativeObservabilityService) InboundContextDecorators() []func(context.Context, binding.Message) context.Context {
	return k.base.InboundContextDecorators()
}

func (k knativeObservabilityService) RecordReceivedMalformedEvent(ctx context.Context, err error) {
	k.base.RecordReceivedMalformedEvent(ctx, err)
}

func (k knativeObservabilityService) RecordCallingInvoker(ctx context.Context, event *event.Event) (context.Context, func(errOrResult error)) {
	return k.base.RecordCallingInvoker(ctx, event)
}

func (k knativeObservabilityService) RecordSendingEvent(ctx context.Context, event event.Event) (context.Context, func(errOrResult error)) {
	ctx, r := obsclient.NewReporter(ctx, reportSend)

	// From https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/trace/semantic_conventions/messaging.md#span-name
	// The span name SHOULD be set to the message destination name and the operation being performed
	spanName := ceobs.ClientSpanName
	spanKind := trace.SpanKindClient

	spanData := observability.SpanDataFromContext(ctx)
	if spanData != nil {
		spanName = spanData.Name
		spanKind = spanData.Kind
	}

	ctx, span := trace.StartSpan(ctx, spanName, trace.WithSpanKind(spanKind))
	span.AddAttributes(obsclient.EventTraceAttributes(&event)...)
	if spanData != nil && len(spanData.Attributes) > 0 {
		span.AddAttributes(spanData.Attributes...)
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

func (k knativeObservabilityService) RecordRequestEvent(ctx context.Context, e event.Event) (context.Context, func(errOrResult error, event *event.Event)) {
	return k.base.RecordRequestEvent(ctx, e)
}
