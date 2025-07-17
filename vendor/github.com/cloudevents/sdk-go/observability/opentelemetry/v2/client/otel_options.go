/*
 Copyright 2021 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"go.opentelemetry.io/otel/attribute"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/observability"
)

const (
	// The value for the `otel.library.name` span attribute
	instrumentationName = "github.com/cloudevents/sdk-go/observability/opentelemetry/v2"
)

type OTelObservabilityServiceOption func(*OTelObservabilityService)

// WithSpanAttributesGetter appends the returned attributes from the function to the span.
func WithSpanAttributesGetter(attrGetter func(cloudevents.Event) []attribute.KeyValue) OTelObservabilityServiceOption {
	return func(os *OTelObservabilityService) {
		if attrGetter != nil {
			os.spanAttributesGetter = attrGetter
		}
	}
}

// WithSpanNameFormatter replaces the default span name with the string returned from the function
func WithSpanNameFormatter(nameFormatter func(cloudevents.Event) string) OTelObservabilityServiceOption {
	return func(os *OTelObservabilityService) {
		if nameFormatter != nil {
			os.spanNameFormatter = nameFormatter
		}
	}
}

var defaultSpanNameFormatter func(cloudevents.Event) string = func(e cloudevents.Event) string {
	return observability.ClientSpanName + "." + e.Context.GetType()
}
