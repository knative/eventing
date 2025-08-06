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
	"context"

	"github.com/cloudevents/sdk-go/v2/binding"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ceCarrier struct {
	reader binding.MessageMetadataReader
	writer binding.MessageMetadataWriter
}

func (c ceCarrier) Get(key string) string {
	// Cloudevents may store values in non-string ways, so we will ignore non-string values
	if val, ok := c.reader.GetExtension(key).(string); ok {
		return val
	}
	return ""
}

func (c ceCarrier) Set(key, value string) {
	c.writer.SetExtension(key, value)
}
func (c ceCarrier) Keys() []string {
	// We really only care about opentelemetry headers, and there is no way to get the keys given the interface we have.
	// So we will just return the keys we care about.
	return (propagation.TraceContext{}).Fields()
}

func PopulateCEDistributedTracing(ctx context.Context) binding.TransformerFunc {
	return func(reader binding.MessageMetadataReader, writer binding.MessageMetadataWriter) error {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			carrier := ceCarrier{
				reader: reader,
				writer: writer,
			}
			otel.GetTextMapPropagator().Inject(ctx, carrier)
		}
		return nil
	}
}
