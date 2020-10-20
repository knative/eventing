package gochan

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

// Sender implements Sender by sending Messages on a channel.
type Sender chan<- binding.Message

func (s Sender) Send(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (err error) {
	if ctx == nil {
		return fmt.Errorf("nil Context")
	} else if m == nil {
		return fmt.Errorf("nil Message")
	}

	defer func() {
		err2 := m.Finish(err)
		if err == nil {
			err = err2
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s <- m:
		return nil
	}
}

func (s Sender) Close(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("trying to close a closed Sender")
		}
	}()
	close(s)
	return nil
}

var _ protocol.SendCloser = (Sender)(nil)
