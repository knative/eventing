/*
Copyright 2018 The Knative Authors

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
	"testing"
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
			append:   []string{"name.ns.service.local"},
			expected: "name.ns.service.local",
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
			m := Message{}
			if tc.start != "" {
				m.Headers = make(map[string]string)
				m.Headers[MessageHistoryHeader] = tc.start
			}
			if tc.set != nil {
				m.setHistory(tc.set)
			}
			for _, name := range tc.append {
				m.AppendToHistory(name)
			}
			history := m.History()
			if len(history) != tc.len {
				t.Errorf("Unexpected number of elements. Want %d, got %d", tc.len, len(history))
			}
			if m.Headers[MessageHistoryHeader] != tc.expected {
				t.Errorf("Unexpected history. Want %q, got %q", tc.expected, m.Headers[MessageHistoryHeader])
			}
		})
	}
}
