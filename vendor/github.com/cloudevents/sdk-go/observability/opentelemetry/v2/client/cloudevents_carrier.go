/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"context"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"go.opentelemetry.io/otel/propagation"
)

// CloudEventCarrier wraps the distributed trace extension to satisfy the TextMapCarrier interface.
// https://github.com/open-telemetry/opentelemetry-go/blob/v1.0.0-RC3/propagation/propagation.go#L23
type CloudEventCarrier struct {
	Extension *extensions.DistributedTracingExtension
}

// NewCloudEventCarrier creates a new CloudEventCarrier with an empty distributed tracing extension.
func NewCloudEventCarrier() CloudEventCarrier {
	return CloudEventCarrier{Extension: &extensions.DistributedTracingExtension{}}
}

// NewCloudEventCarrierWithEvent creates a new CloudEventCarrier with a distributed tracing extension
// populated with the trace data from the event.
func NewCloudEventCarrierWithEvent(ctx context.Context, event cloudevents.Event) CloudEventCarrier {
	var te, ok = extensions.GetDistributedTracingExtension(event)
	if !ok {
		cecontext.LoggerFrom(ctx).Warn("Could not get the distributed tracing extension from the event.")
		return CloudEventCarrier{Extension: &extensions.DistributedTracingExtension{}}
	}
	return CloudEventCarrier{Extension: &te}
}

// Get returns the value associated with the passed key.
func (cec CloudEventCarrier) Get(key string) string {
	switch key {
	case extensions.TraceParentExtension:
		return cec.Extension.TraceParent
	case extensions.TraceStateExtension:
		return cec.Extension.TraceState
	default:
		return ""
	}
}

// Set stores the key-value pair.
func (cec CloudEventCarrier) Set(key string, value string) {
	switch key {
	case extensions.TraceParentExtension:
		cec.Extension.TraceParent = value
	case extensions.TraceStateExtension:
		cec.Extension.TraceState = value
	}
}

// Keys lists the keys stored in this carrier.
func (cec CloudEventCarrier) Keys() []string {
	return []string{extensions.TraceParentExtension, extensions.TraceStateExtension}
}

// InjectDistributedTracingExtension injects the tracecontext from the context into the event as a DistributedTracingExtension
//
// If a DistributedTracingExtension is present in the provided event, its current value is replaced with the
// tracecontext obtained from the context.
func InjectDistributedTracingExtension(ctx context.Context, event cloudevents.Event) {
	tc := propagation.TraceContext{}
	carrier := NewCloudEventCarrier()
	tc.Inject(ctx, carrier)
	carrier.Extension.AddTracingAttributes(&event)
}

// ExtractDistributedTracingExtension extracts the tracecontext from the cloud event into the context.
//
// Calling this method will always replace the tracecontext in the context with the one extracted from the event.
// In case this is undesired, check first if the context has a recording span with: `trace.SpanFromContext(ctx)`
func ExtractDistributedTracingExtension(ctx context.Context, event cloudevents.Event) context.Context {
	tc := propagation.TraceContext{}
	carrier := NewCloudEventCarrierWithEvent(ctx, event)

	return tc.Extract(ctx, carrier)
}
