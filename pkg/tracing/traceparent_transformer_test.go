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
	"testing"

	"github.com/cloudevents/sdk-go/pkg/binding/test"
	"go.opencensus.io/trace"
)

func TestTraceparentTransformer(t *testing.T) {
	_, span := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.NeverSample()))
	_, newSpan := trace.StartSpan(context.Background(), "name", trace.WithSampler(trace.NeverSample()))

	withoutTraceparent := test.MinEvent()
	withoutTraceparentExpected := withoutTraceparent.Clone()
	withoutTraceparentExpected.SetExtension(traceparentAttribute, traceparentAttributeValue(span))

	withTraceparent := test.MinEvent()
	withTraceparent.SetExtension(traceparentAttribute, traceparentAttributeValue(span))
	withTraceparentExpected := withTraceparent.Clone()
	withTraceparentExpected.SetExtension(traceparentAttribute, traceparentAttributeValue(newSpan))

	test.RunTransformerTests(t, context.Background(), []test.TransformerTestArgs{
		{
			Name:         "Add traceparent in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withoutTraceparent),
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Add traceparent in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withoutTraceparent),
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Add traceparent in Event message",
			InputEvent:   withoutTraceparent,
			WantEvent:    withoutTraceparentExpected,
			Transformers: AddTraceparent(span),
		},
		{
			Name:         "Update traceparent in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withTraceparent),
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
		{
			Name:         "Update traceparent in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withTraceparent),
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
		{
			Name:         "Update traceparent in Event message",
			InputEvent:   withTraceparent,
			WantEvent:    withTraceparentExpected,
			Transformers: AddTraceparent(newSpan),
		},
	})
}
