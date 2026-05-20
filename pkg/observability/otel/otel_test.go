/*
Copyright 2026 The Knative Authors

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

package otel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
)

func TestMetricAttributesDenyFilter(t *testing.T) {
	view := metricAttributesDenyFilter([]string{"cloudevents.type", "messaging.destination.name"})

	stream, ok := view(metric.Instrument{Name: "kn.eventing.dispatch.duration"})
	assert.True(t, ok, "view should match kn.eventing.* instruments")
	assert.NotNil(t, stream.AttributeFilter)

	denied := []attribute.KeyValue{
		attribute.String("cloudevents.type", "com.example.event"),
		attribute.String("messaging.destination.name", "my-destination"),
	}
	for _, kv := range denied {
		assert.False(t, stream.AttributeFilter(kv), "attribute %s should be denied", kv.Key)
	}

	allowed := []attribute.KeyValue{
		attribute.String("messaging.system", "knative"),
		attribute.Int("http.response.status_code", 200),
	}
	for _, kv := range allowed {
		assert.True(t, stream.AttributeFilter(kv), "attribute %s should be allowed", kv.Key)
	}
}

func TestMetricAttributesDenyFilterDoesNotMatchOtherInstruments(t *testing.T) {
	view := metricAttributesDenyFilter([]string{"cloudevents.type"})

	_, ok := view(metric.Instrument{Name: "http.server.request.duration"})
	assert.False(t, ok, "view should not match non kn.eventing.* instruments")
}
