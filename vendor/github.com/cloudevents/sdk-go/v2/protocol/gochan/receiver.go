package gochan

import (
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"io"

	"github.com/cloudevents/sdk-go/v2/binding"
)

// Receiver implements Receiver by receiving Messages from a channel.
type Receiver <-chan binding.Message

func (r Receiver) Receive(ctx context.Context) (binding.Message, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil Context")
	}

	select {
	case <-ctx.Done():
		return nil, io.EOF
	case m, ok := <-r:
		if !ok {
			return nil, io.EOF
		}
		return m, nil
	}
}

var _ protocol.Receiver = (*Receiver)(nil)
