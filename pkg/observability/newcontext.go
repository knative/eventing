/*
Copyright 2025 The Knative Authors

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

package observability

import (
	"context"
	"net/http"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/types"
)

func WithLabeler(ctx context.Context) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	return ctx
}

func MessagingLabels(destination, operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		MessagingDestinationName.With(destination),
		MessagingOperationName.With(operation),
		MessagingSystem.With(KnativeMessagingSystem),
	}
}

func WithMessagingLabels(ctx context.Context, destination, operation string) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(
		MessagingLabels(destination, operation)...,
	)

	return ctx
}

func WithLowCardinalityMessagingLabels(ctx context.Context, destinationTemplate, operation string) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(
		MessagingDestinationTemplate.With(destinationTemplate),
		MessagingOperationName.With(operation),
		MessagingSystem.With(KnativeMessagingSystem),
	)

	return ctx
}

func WithRequestLabels(ctx context.Context, r *http.Request) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	labeler.Add(RequestScheme.With(scheme))

	return ctx
}

// WithMinimalEventLabels adds a minimal set of event labels, suitable for metrics (to keep cardinality lower)
func WithMinimalEventLabels(ctx context.Context, event *cloudevents.Event) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(CloudEventType.With(event.Type()))

	return ctx
}

func WithEventLabels(ctx context.Context, event *cloudevents.Event) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labels := []attribute.KeyValue{
		CloudEventType.With(event.Type()),
		CloudEventSource.With(event.Source()),
		CloudEventSpecVersion.With(event.SpecVersion()),
	}

	if subject := event.Subject(); subject != "" {
		labels = append(labels, CloudEventSubject.With(subject))
	}

	if dataContentType := event.DataContentType(); dataContentType != "" {
		labels = append(labels, CloudEventDataContenttype.With(dataContentType))
	}

	labeler.Add(labels...)

	return ctx
}

func WithChannelLabels(ctx context.Context, channel types.NamespacedName) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(
		ChannelName.With(channel.Name),
		ChannelNamespace.With(channel.Namespace),
	)

	return ctx
}

func WithBrokerLabels(ctx context.Context, broker types.NamespacedName) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(
		BrokerName.With(broker.Name),
		BrokerNamespace.With(broker.Namespace),
	)

	return ctx
}

func WithSinkLabels(ctx context.Context, sink types.NamespacedName, kind string) context.Context {
	labeler, ok := otelhttp.LabelerFromContext(ctx)
	if !ok {
		ctx = otelhttp.ContextWithLabeler(ctx, labeler)
	}

	labeler.Add(
		SinkName.With(sink.Name),
		SinkNamespace.With(sink.Namespace),
		SinkKind.With(kind),
	)

	return ctx
}
