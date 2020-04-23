package buffering

import (
	"sync/atomic"

	"github.com/cloudevents/sdk-go/v2/binding"
)

type acksMessage struct {
	binding.Message
	requiredAcks int32
}

func (m *acksMessage) GetWrappedMessage() binding.Message {
	return m.Message
}

func (m *acksMessage) Finish(err error) error {
	remainingAcks := atomic.AddInt32(&m.requiredAcks, -1)
	if remainingAcks == 0 {
		return m.Message.Finish(err)
	}
	return nil
}

var _ binding.MessageWrapper = (*acksMessage)(nil)

// WithAcksBeforeFinish returns a wrapper for m that calls m.Finish()
// only after the specified number of acks are received.
// Use it when you need to dispatch a Message using several Sender instances
func WithAcksBeforeFinish(m binding.Message, requiredAcks int) binding.Message {
	return &acksMessage{Message: m, requiredAcks: int32(requiredAcks)}
}
