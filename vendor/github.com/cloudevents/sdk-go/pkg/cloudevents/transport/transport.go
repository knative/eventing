package transport

import (
	"context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

// Transport is the interface for transport sender to send the converted Message
// over the underlying transport.
type Sender interface {
	Send(context.Context, cloudevents.Event) error
}

// Receiver TODO not sure yet.
type Receiver interface {
	Receive(cloudevents.Event)
}
