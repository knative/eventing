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

package channel

import (
	"context"
	"testing"

	"github.com/cloudevents/sdk-go/v2/binding/test"
)

func TestHistoryTransformer(t *testing.T) {
	withoutHistory := test.MinEvent()
	withoutHistoryExpected := withoutHistory.Clone()
	withoutHistoryExpected.SetExtension(EventHistory, "example.com")

	withHistory := test.MinEvent()
	withHistory.SetExtension(EventHistory, "knative.dev")
	withHistoryExpected := withHistory.Clone()
	withHistoryExpected.SetExtension(EventHistory, encodeEventHistory([]string{"knative.dev", "example.com"}))

	test.RunTransformerTests(t, context.Background(), []test.TransformerTestArgs{
		{
			Name:         "Add history in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withoutHistory),
			WantEvent:    withoutHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
		{
			Name:         "Add history in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withoutHistory),
			WantEvent:    withoutHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
		{
			Name:         "Add history in Event message",
			InputEvent:   withoutHistory,
			WantEvent:    withoutHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
		{
			Name:         "Update history in Mock Structured message",
			InputMessage: test.MustCreateMockStructuredMessage(withHistory),
			WantEvent:    withHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
		{
			Name:         "Update history in Mock Binary message",
			InputMessage: test.MustCreateMockBinaryMessage(withHistory),
			WantEvent:    withHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
		{
			Name:         "Update history in Event message",
			InputEvent:   withHistory,
			WantEvent:    withHistoryExpected,
			Transformers: AddHistory("example.com"),
		},
	})
}
