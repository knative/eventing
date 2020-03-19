package binding

import (
	"context"
	"errors"
	"io"
)

// ChanSender implements Sender by sending Messages on a channel.
type ChanSender chan<- Message

// ChanReceiver implements Receiver by receiving Messages from a channel.
type ChanReceiver <-chan Message

func (s ChanSender) Send(ctx context.Context, m Message) (err error) {
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

func (s ChanSender) Close(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("send on closed channel")
		}
	}()
	close(s)
	return nil
}

func (r ChanReceiver) Receive(ctx context.Context) (Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case m, ok := <-r:
		if !ok {
			return nil, io.EOF
		}
		return m, nil
	}
}

func (r ChanReceiver) Close(ctx context.Context) error { return nil }
