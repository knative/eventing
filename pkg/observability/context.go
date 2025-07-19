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

package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

type spanDataKey struct{}

// SpanData contains values for creating tracing spans.
type SpanData struct {
	// Name is the span name
	Name string

	// Kind is the span kind
	Kind int

	// Attributes is the additional set of span attributes
	Attributes []attribute.KeyValue
}

// WithSpanData extends the given context with the given span values
func WithSpanData(ctx context.Context, name string, kind int, attributes []attribute.KeyValue) context.Context {
	return context.WithValue(ctx, spanDataKey{}, &SpanData{
		Name:       name,
		Kind:       kind,
		Attributes: attributes,
	})
}

// SpanDataFromContext gets the span values from the context
func SpanDataFromContext(ctx context.Context) *SpanData {
	val := ctx.Value(spanDataKey{})
	if val == nil {
		return nil
	}
	return val.(*SpanData)
}
