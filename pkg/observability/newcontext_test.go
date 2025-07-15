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
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/observability/attributekey"
)

func TestWithMessagingLabels(t *testing.T) {
	tt := map[string]struct {
		initialLabels []attribute.KeyValue
		destination   string
		operation     string
		want          []attribute.KeyValue
	}{
		"values set properly": {
			destination: "default.broker",
			operation:   "send",
			want: []attribute.KeyValue{
				MessagingDestinationName.With("default.broker"),
				MessagingOperationName.With("send"),
				MessagingSystem.With(KnativeMessagingSystem),
			},
		},
		"other labels not clobbered": {
			initialLabels: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
			},
			destination: "default.broker",
			operation:   "send",
			want: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
				MessagingDestinationName.With("default.broker"),
				MessagingOperationName.With("send"),
				MessagingSystem.With(KnativeMessagingSystem),
			},
		},
	}

	for tName, tCase := range tt {
		t.Run(tName, func(t *testing.T) {
			ctx := context.Background()
			if tCase.initialLabels != nil {
				labeler := &otelhttp.Labeler{}
				labeler.Add(tCase.initialLabels...)
				ctx = otelhttp.ContextWithLabeler(ctx, labeler)
			}

			ctx = WithMessagingLabels(ctx, tCase.destination, tCase.operation)

			labeler, ok := otelhttp.LabelerFromContext(ctx)
			assert.True(t, ok, "labeler should be set on the context")
			assert.ElementsMatch(t, tCase.want, labeler.Get())
		})
	}
}

func TestWithEventLabels(t *testing.T) {
	sampleEvent := cloudevents.NewEvent()
	sampleEvent.SetID("123456789")
	sampleEvent.SetSource("http://localhost:8080")
	sampleEvent.SetDataContentType("application/json")
	sampleEvent.SetSubject("{productId}")
	sampleEvent.SetType("example.event")

	tt := map[string]struct {
		initialLabels []attribute.KeyValue
		event         cloudevents.Event
		want          []attribute.KeyValue
	}{
		"values set properly": {
			event: sampleEvent,
			want: []attribute.KeyValue{
				CloudEventDataContenttype.With("application/json"),
				CloudEventSource.With("http://localhost:8080"),
				CloudEventSpecVersion.With(cloudevents.VersionV1),
				CloudEventSubject.With("{productId}"),
				CloudEventType.With("example.event"),
			},
		},
		"other labels not clobbered": {
			initialLabels: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
			},
			event: sampleEvent,
			want: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
				CloudEventDataContenttype.With("application/json"),
				CloudEventSource.With("http://localhost:8080"),
				CloudEventSpecVersion.With(cloudevents.VersionV1),
				CloudEventSubject.With("{productId}"),
				CloudEventType.With("example.event"),
			},
		},
	}

	for tName, tCase := range tt {
		t.Run(tName, func(t *testing.T) {
			ctx := context.Background()
			if tCase.initialLabels != nil {
				labeler := &otelhttp.Labeler{}
				labeler.Add(tCase.initialLabels...)
				ctx = otelhttp.ContextWithLabeler(ctx, labeler)
			}

			ctx = WithEventLabels(ctx, tCase.event)

			labeler, ok := otelhttp.LabelerFromContext(ctx)
			assert.True(t, ok, "labeler should be set on the context")
			assert.ElementsMatch(t, tCase.want, labeler.Get())
		})
	}
}

func TestWithBrokerLabels(t *testing.T) {
	sampleBroker := types.NamespacedName{Name: "broker", Namespace: "default"}

	tt := map[string]struct {
		initialLabels []attribute.KeyValue
		broker        types.NamespacedName
		want          []attribute.KeyValue
	}{
		"values set properly": {
			broker: sampleBroker,
			want: []attribute.KeyValue{
				BrokerName.With("broker"),
				BrokerNamespace.With("default"),
			},
		},
		"other labels not clobbered": {
			initialLabels: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
			},
			broker: sampleBroker,
			want: []attribute.KeyValue{
				attributekey.Int("example.value").With(42),
				BrokerName.With("broker"),
				BrokerNamespace.With("default"),
			},
		},
	}

	for tName, tCase := range tt {
		t.Run(tName, func(t *testing.T) {
			ctx := context.Background()
			if tCase.initialLabels != nil {
				labeler := &otelhttp.Labeler{}
				labeler.Add(tCase.initialLabels...)
				ctx = otelhttp.ContextWithLabeler(ctx, labeler)
			}

			ctx = WithBrokerLabels(ctx, tCase.broker)

			labeler, ok := otelhttp.LabelerFromContext(ctx)
			assert.True(t, ok, "labeler should be set on the context")
			assert.ElementsMatch(t, tCase.want, labeler.Get())
		})
	}
}
