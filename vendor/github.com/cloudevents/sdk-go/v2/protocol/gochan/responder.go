package gochan

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

type ChanResponderResponse struct {
	Message binding.Message
	Result  protocol.Result
}

// Responder implements Responder by receiving Messages from a channel and outputting the result in an output channel.
// All message received in the `Out` channel must be finished
type Responder struct {
	In  <-chan binding.Message
	Out chan<- ChanResponderResponse
}

func (r *Responder) Respond(ctx context.Context) (binding.Message, protocol.ResponseFn, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("nil Context")
	}

	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case m, ok := <-r.In:
		if !ok {
			return nil, nil, io.EOF
		}
		return m, func(ctx context.Context, message binding.Message, result protocol.Result, transformers ...binding.Transformer) error {
			r.Out <- ChanResponderResponse{
				Message: message,
				Result:  result,
			}
			return nil
		}, nil
	}
}

var _ protocol.Responder = (*Responder)(nil)
