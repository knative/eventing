package gochan

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

type Requester struct {
	Ch    chan<- binding.Message
	Reply func(message binding.Message) (binding.Message, error)
}

func (s *Requester) Send(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (err error) {
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
	case s.Ch <- m:
		return nil
	}
}

func (s *Requester) Request(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (res binding.Message, err error) {
	defer func() {
		err2 := m.Finish(err)
		if err == nil {
			err = err2
		}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case s.Ch <- m:
		return s.Reply(m)
	}
}

func (s *Requester) Close(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("trying to close a closed Sender")
		}
	}()
	close(s.Ch)
	return nil
}

var _ protocol.RequesterCloser = (*Requester)(nil)
