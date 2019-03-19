package transport

import (
	"context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

// Transport is the interface for transport sender to send the converted Message
// over the underlying transport.
type Transport interface {
	Send(context.Context, cloudevents.Event) (*cloudevents.Event, error)

	SetReceiver(Receiver)
	StartReceiver(context.Context) error
}

type Receiver interface {
	Receive(context.Context, cloudevents.Event, *cloudevents.EventResponse) error
}
