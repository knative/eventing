/*
Copyright 2023 The Knative Authors

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

package new_trigger_filters

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilterPrint(t *testing.T) {
	tt := []struct {
		filter   Filter
		expected string
		name     string
	}{
		{
			name: "prefix filter",
			filter: &AttributeFilter{
				Type:      "prefix",
				Attribute: "type",
				Value:     "com.github",
			},
			expected: "- prefix:\n\ttype: com.github",
		},
		{
			name: "suffix filter",
			filter: &AttributeFilter{
				Type:      "suffix",
				Attribute: "type",
				Value:     ".created",
			},
			expected: "- suffix:\n\ttype: .created",
		},
		{
			name: "exact filter",
			filter: &AttributeFilter{
				Type:      "exact",
				Attribute: "type",
				Value:     "com.github.push",
			},
			expected: "- exact:\n\ttype: com.github.push",
		},
		{
			name: "not filter",
			filter: &NotFilter{
				Filter: &AttributeFilter{
					Type:      "exact",
					Attribute: "type",
					Value:     "com.github.push",
				},
			},
			expected: "- not:\n\t- exact:\n\t\ttype: com.github.push",
		},
		{
			name: "any filter",
			filter: &ArrayFilter{
				Type: "any",
				Filters: []Filter{
					&AttributeFilter{
						Type:      "suffix",
						Attribute: "type",
						Value:     ".created",
					},
					&NotFilter{
						Filter: &AttributeFilter{
							Type:      "exact",
							Attribute: "type",
							Value:     "com.github.push",
						},
					},
				},
			},
			expected: "- any:\n\t- suffix:\n\t\ttype: .created\n\t- not:\n\t\t- exact:\n\t\t\ttype: com.github.push",
		},
		{
			name: "all filter",
			filter: &ArrayFilter{
				Type: "all",
				Filters: []Filter{
					&AttributeFilter{
						Type:      "suffix",
						Attribute: "type",
						Value:     ".created",
					},
					&NotFilter{
						Filter: &AttributeFilter{
							Type:      "exact",
							Attribute: "type",
							Value:     "com.github.push",
						},
					},
				},
			},
			expected: "- all:\n\t- suffix:\n\t\ttype: .created\n\t- not:\n\t\t- exact:\n\t\t\ttype: com.github.push",
		},
	}

	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			filterString := tc.filter.FilterString(0)
			require.Equal(t, tc.expected, filterString)
		})
	}

}
