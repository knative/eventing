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

	"github.com/cloudevents/sdk-go/v2/binding"
	bindingtest "github.com/cloudevents/sdk-go/v2/binding/test"
	"github.com/cloudevents/sdk-go/v2/test"
)

func TestMessageHistory(t *testing.T) {
	var cases = []struct {
		start    string
		set      []string
		append   []string
		expected string
		len      int
	}{
		{
			expected: "",
			len:      0,
		},
		{
			append:   []string{"name.Ns.service.local"},
			expected: "name.Ns.service.local",
			len:      1,
		},
		{
			append:   []string{"name.withspace.service.local  "},
			expected: "name.withspace.service.local",
			len:      1,
		},
		{
			append:   []string{"name1.ns1.service.local", "name2.ns2.service.local"},
			expected: "name1.ns1.service.local; name2.ns2.service.local",
			len:      2,
		},
		{
			start:    "name1.ns1.service.local",
			append:   []string{"name2.ns2.service.local", "name3.ns3.service.local"},
			expected: "name1.ns1.service.local; name2.ns2.service.local; name3.ns3.service.local",
			len:      3,
		},
		{
			start:    "name1.ns1.service.local; name2.ns2.service.local",
			append:   []string{"nameadd.nsadd.service.local"},
			expected: "name1.ns1.service.local; name2.ns2.service.local; nameadd.nsadd.service.local",
			len:      3,
		},
		{
			start:    "name1.ns1.service.local; name2.ns2.service.local",
			set:      []string{"name3.ns3.service.local"},
			expected: "name3.ns3.service.local",
			len:      1,
		},
		{
			start:    "  ",
			append:   []string{"name1.ns1.service.local"},
			expected: "name1.ns1.service.local",
			len:      1,
		},
		{
			start:    "  ",
			append:   []string{" ", "name.multispace.service.local", "  "},
			expected: "name.multispace.service.local",
			len:      1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			val := encodeEventHistory(decodeEventHistory(tc.start))
			if tc.set != nil {
				val = encodeEventHistory(tc.set)
			}
			for _, name := range tc.append {
				val = encodeEventHistory(append(decodeEventHistory(val), decodeEventHistory(name)...))
			}
			h := decodeEventHistory(val)
			if len(h) != tc.len {
				t.Errorf("Unexpected number of elements. Want %d, got %d", tc.len, len(h))
			}
			if val != tc.expected {
				t.Errorf("Unexpected history. Want %q, got %q", tc.expected, val)
			}
		})
	}
}

func TestHistoryTransformer(t *testing.T) {
	withoutHistory := test.MinEvent()
	withoutHistoryExpected := withoutHistory.Clone()
	withoutHistoryExpected.SetExtension(EventHistory, "example.com")

	withHistory := test.MinEvent()
	withHistory.SetExtension(EventHistory, "knative.dev")
	withHistoryExpected := withHistory.Clone()
	withHistoryExpected.SetExtension(EventHistory, encodeEventHistory([]string{"knative.dev", "example.com"}))

	bindingtest.RunTransformerTests(t, context.Background(), []bindingtest.TransformerTestArgs{
		{
			Name:         "Add history in Mock Structured message",
			InputMessage: bindingtest.MustCreateMockStructuredMessage(t, withoutHistory),
			WantEvent:    withoutHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
		{
			Name:         "Add history in Mock Binary message",
			InputMessage: bindingtest.MustCreateMockBinaryMessage(withoutHistory),
			WantEvent:    withoutHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
		{
			Name:         "Add history in Event message",
			InputEvent:   withoutHistory,
			WantEvent:    withoutHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
		{
			Name:         "Update history in Mock Structured message",
			InputMessage: bindingtest.MustCreateMockStructuredMessage(t, withHistory),
			WantEvent:    withHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
		{
			Name:         "Update history in Mock Binary message",
			InputMessage: bindingtest.MustCreateMockBinaryMessage(withHistory),
			WantEvent:    withHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
		{
			Name:         "Update history in Event message",
			InputEvent:   withHistory,
			WantEvent:    withHistoryExpected,
			Transformers: binding.Transformers{AddHistory("example.com")},
		},
	})
}
