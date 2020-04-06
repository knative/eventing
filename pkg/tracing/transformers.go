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

package tracing

import (
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
	"github.com/cloudevents/sdk-go/v2/binding/transformer"
	"github.com/cloudevents/sdk-go/v2/client"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"go.opencensus.io/trace"
)

// AddTraceparent returns a CloudEvent that is identical to the input event,
// with the traceparent CloudEvents extension attribute added.
func AddTraceparent(span *trace.Span) []binding.TransformerFactory {
	dt := extensions.FromSpanContext(span.SpanContext())
	return transformer.SetExtension(extensions.TraceParentExtension, dt.TraceParent, func(i2 interface{}) (i interface{}, err error) {
		return dt.TraceParent, nil
	})
}

func PopulateSpan(span *trace.Span) binding.TransformerFactory {
	return populateSpanTransformerFactory{span: span}
}

type populateSpanTransformerFactory struct {
	span *trace.Span
}

func (v populateSpanTransformerFactory) StructuredTransformer(binding.StructuredWriter) binding.StructuredWriter {
	return nil // Not supported, must fallback to EventTransformer!
}

func (v populateSpanTransformerFactory) BinaryTransformer(encoder binding.BinaryWriter) binding.BinaryWriter {
	return &binaryPopulateSpanTransformer{BinaryWriter: encoder, span: v.span}
}

func (v populateSpanTransformerFactory) EventTransformer() binding.EventTransformer {
	return func(e *event.Event) error {
		v.span.AddAttributes(client.EventTraceAttributes(e)...)
		v.span.AddAttributes(trace.StringAttribute("cloudevents.id", e.ID()))
		return nil
	}
}

type binaryPopulateSpanTransformer struct {
	binding.BinaryWriter
	span *trace.Span
}

func (b *binaryPopulateSpanTransformer) SetAttribute(attribute spec.Attribute, value interface{}) error {
	if s, ok := value.(string); ok {
		b.span.AddAttributes(trace.StringAttribute("cloudevents."+attribute.Name(), s))
	}
	return b.BinaryWriter.SetAttribute(attribute, value)
}
