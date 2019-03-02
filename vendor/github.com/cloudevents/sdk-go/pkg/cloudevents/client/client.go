package client

import (
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/nats"
)

type Receiver func(event cloudevents.Event)

type Client interface {
	Send(ctx context.Context, event cloudevents.Event) error
	StartReceiver(ctx context.Context, fn Receiver) error

	Receive(event cloudevents.Event)
}

type ceClient struct {
	transport transport.Sender
	receiver  Receiver

	eventDefaulterFns []EventDefaulter
}

func (c *ceClient) Send(ctx context.Context, event cloudevents.Event) error {
	// Confirm we have a transport set.
	if c.transport == nil {
		return fmt.Errorf("client not ready, transport not initalized")
	}
	// Apply the defaulter chain to the incoming event.
	if len(c.eventDefaulterFns) > 0 {
		for _, fn := range c.eventDefaulterFns {
			event = fn(event)
		}
	}
	// Validate the event conforms to the CloudEvents Spec.
	if err := event.Validate(); err != nil {
		return err
	}
	// Send the event over the transport.
	return c.transport.Send(ctx, event)
}

func (c *ceClient) Receive(event cloudevents.Event) {
	if c.receiver != nil {
		c.receiver(event)
	}
}

func (c *ceClient) StartReceiver(ctx context.Context, fn Receiver) error {
	if c.transport == nil {
		return fmt.Errorf("client not ready, transport not initalized")
	}

	if t, ok := c.transport.(*http.Transport); ok {
		return c.startHTTPReceiver(ctx, t, fn)
	}

	if t, ok := c.transport.(*nats.Transport); ok {
		return c.startNATSReceiver(ctx, t, fn)
	}

	return fmt.Errorf("unknown transport type: %T", c.transport)
}

func (c *ceClient) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}
