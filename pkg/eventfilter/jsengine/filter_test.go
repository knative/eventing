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
