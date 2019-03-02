package nats

import (
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/nats-io/go-nats"
	"log"
)

// type check that this transport message impl matches the contract
var _ transport.Sender = (*Transport)(nil)

// Transport acts as both a http client and a http handler.
type Transport struct {
	Encoding Encoding
	Conn     *nats.Conn
	Subject  string

	sub *nats.Subscription

	Receiver transport.Receiver

	codec transport.Codec
}

func (t *Transport) loadCodec() bool {
	if t.codec == nil {
		switch t.Encoding {
		case Default:
			t.codec = &Codec{}
		case StructuredV02:
			t.codec = &CodecV02{Encoding: t.Encoding}
		case StructuredV03:
			t.codec = &CodecV03{Encoding: t.Encoding}
		default:
			return false
		}
	}
	return true
}

func (t *Transport) Send(ctx context.Context, event cloudevents.Event) error {
	if ok := t.loadCodec(); !ok {
		return fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
	}

	msg, err := t.codec.Encode(event)
	if err != nil {
		return err
	}

	if m, ok := msg.(*Message); ok {
		return t.Conn.Publish(t.Subject, m.Body)
	}

	return fmt.Errorf("failed to encode Event into a Message")
}

func (t *Transport) Listen(ctx context.Context) error {
	if t.sub != nil {
		return fmt.Errorf("already subscribed")
	}

	if ok := t.loadCodec(); !ok {
		return fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
	}

	// TODO: there could be more than one subscription. Might have to do a map
	// of subject to subscription.

	if t.Subject == "" {
		return fmt.Errorf("subject required for nats listen")
	}

	var err error
	// Simple Async Subscriber
	t.sub, err = t.Conn.Subscribe(t.Subject, func(m *nats.Msg) {
		msg := &Message{
			Body: m.Data,
		}
		event, err := t.codec.Decode(msg)
		if err != nil {
			log.Printf("failed to decode message: %s", err)
			return
		}
		t.Receiver.Receive(*event)
	})
	return err
}
