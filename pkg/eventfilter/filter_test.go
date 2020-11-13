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

package eventfilter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/stretchr/testify/require"
)

type mockFilter FilterResult

func (p mockFilter) Filter(ctx context.Context, event cloudevents.Event) FilterResult {
	return FilterResult(p)
}

func TestFilters(t *testing.T) {
	tests := []struct {
		have []FilterResult
		want FilterResult
	}{{
		have: []FilterResult{},
		want: NoFilter,
	}, {
		have: []FilterResult{PassFilter},
		want: PassFilter,
	}, {
		have: []FilterResult{FailFilter},
		want: FailFilter,
	}, {
		have: []FilterResult{PassFilter, PassFilter},
		want: PassFilter,
	}, {
		have: []FilterResult{PassFilter, FailFilter},
		want: FailFilter,
	}, {
		have: []FilterResult{FailFilter, FailFilter},
		want: FailFilter,
	}}
	for _, tt := range tests {
		t.Run(testName(tt.have, tt.want), func(t *testing.T) {
			var filters Filters
			for _, fr := range tt.have {
				filters = append(filters, mockFilter(fr))
			}
			require.Equal(t, tt.want, filters.Filter(context.TODO(), cloudevents.Event{}))
		})
	}
}

func testName(res []FilterResult, want FilterResult) string {
	if len(res) != 0 {
		var operands []string
		for _, r := range res {
			operands = append(operands, fmt.Sprintf("'%s'", string(r)))
		}
		return strings.Join(operands, " and ") + " = '" + string(want) + "'"
	}
	return string(NoFilter) + " = '" + string(want) + "'"
}
