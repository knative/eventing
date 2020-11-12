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

package jsengine

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/stretchr/testify/require"

	"knative.dev/eventing/pkg/eventfilter"
)

func TestJsFilter(t *testing.T) {
	event := test.FullEvent()

	tests := []struct {
		expression string
		want       eventfilter.FilterResult
	}{{
		expression: "event.id === \"" + event.ID() + "\"",
		want:       eventfilter.PassFilter,
	}, {
		expression: "event.id !== \"" + event.ID() + "\"",
		want:       eventfilter.FailFilter,
	}, {
		// Syntax valid but something not defined should generate an error at runtime
		expression: "event.id == something",
		want:       eventfilter.FailFilter,
	}}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("eval(%s) = %s", tt.expression, tt.want), func(t *testing.T) {
			filter, err := NewJsFilter(tt.expression)
			require.NoError(t, err)
			require.Equal(t, tt.want, filter.Filter(context.TODO(), event))
		})
	}
}
