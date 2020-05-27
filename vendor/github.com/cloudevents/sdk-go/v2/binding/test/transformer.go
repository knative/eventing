package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/test"
)

type TransformerTestArgs struct {
	Name         string
	InputEvent   event.Event
	InputMessage binding.Message
	WantEvent    event.Event
	AssertFunc   func(t *testing.T, event event.Event)
	Transformers []binding.Transformer
}

func RunTransformerTests(t *testing.T, ctx context.Context, tests []TransformerTestArgs) {
	for _, tt := range tests {
		tt := tt // Don't use range variable inside scope
		t.Run(tt.Name, func(t *testing.T) {

			var inputMessage binding.Message
			if tt.InputMessage != nil {
				inputMessage = tt.InputMessage
			} else {
				e := tt.InputEvent.Clone()
				inputMessage = (*binding.EventMessage)(&e)
			}

			mockStructured := MockStructuredMessage{}
			mockBinary := MockBinaryMessage{}

			enc, err := binding.Write(ctx, inputMessage, &mockStructured, &mockBinary, tt.Transformers...)
			require.NoError(t, err)

			var e *event.Event
			if enc == binding.EncodingStructured {
				e, err = binding.ToEvent(ctx, &mockStructured)
				require.NoError(t, err)
			} else if enc == binding.EncodingBinary {
				e, err = binding.ToEvent(ctx, &mockBinary)
				require.NoError(t, err)
			} else {
				t.Fatalf("Unexpected encoding %v", enc)
			}
			require.NoError(t, err)
			if tt.AssertFunc != nil {
				tt.AssertFunc(t, *e)
			} else {
				test.AssertEventEquals(t, tt.WantEvent, *e)
			}
		})
	}
}
