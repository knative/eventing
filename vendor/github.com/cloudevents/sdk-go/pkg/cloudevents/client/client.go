package client

import (
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"sync"
)

type Client interface {
	// Send will transmit the given event over the client's configured transport.
	Send(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error)

	// StartReceiver will register the provided function for callback on receipt
	// of a cloudevent. It will also start the underlying transport as it has
	// been configured.
	// This call is blocking.
	// Valid fn signatures are:
	// * func()
	// * func() error
	// * func(context.Context)
	// * func(context.Context) error
	// * func(cloudevents.Event)
	// * func(cloudevents.Event) error
	// * func(context.Context, cloudevents.Event)
	// * func(context.Context, cloudevents.Event) error
	// * func(cloudevents.Event, *cloudevents.EventResponse)
	// * func(cloudevents.Event, *cloudevents.EventResponse) error
	// * func(context.Context, cloudevents.Event, *cloudevents.EventResponse)
	// * func(context.Context, cloudevents.Event, *cloudevents.EventResponse) error
	// Note: if fn returns an error, it is treated as a critical and
	// EventResponse will not be processed.
	StartReceiver(ctx context.Context, fn interface{}) error
}

func New(t transport.Transport, opts ...Option) (Client, error) {
	c := &ceClient{
		transport: t,
	}
	if err := c.applyOptions(opts...); err != nil {
		return nil, err
	}
	t.SetReceiver(c)
	return c, nil
}

// NewDefault provides the good defaults for the common case using an HTTP
// Transport client.
func NewDefault() (Client, error) {
	t, err := http.New(http.WithBinaryEncoding())
	if err != nil {
		return nil, err
	}
	c, err := New(t, WithTimeNow(), WithUUIDs())
	if err != nil {
		return nil, err
	}
	return c, nil
}

type ceClient struct {
	transport transport.Transport
	fn        *receiverFn

	receiverMu        sync.Mutex
	eventDefaulterFns []EventDefaulter
}

func (c *ceClient) Send(ctx context.Context, event cloudevents.Event) (*cloudevents.Event, error) {
	// Confirm we have a transport set.
	if c.transport == nil {
		return nil, fmt.Errorf("client not ready, transport not initialized")
	}
	// Apply the defaulter chain to the incoming event.
	if len(c.eventDefaulterFns) > 0 {
		for _, fn := range c.eventDefaulterFns {
			event = fn(event)
		}
	}
	// Validate the event conforms to the CloudEvents Spec.
	if err := event.Validate(); err != nil {
		return nil, err
	}
	// Send the event over the transport.
	return c.transport.Send(ctx, event)
}

// Receive is called from from the transport on event delivery.
func (c *ceClient) Receive(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	if c.fn != nil {
		err := c.fn.invoke(ctx, event, resp)
		// Apply the defaulter chain to the outgoing event.
		if err == nil && resp != nil && resp.Event != nil && len(c.eventDefaulterFns) > 0 {
			for _, fn := range c.eventDefaulterFns {
				*resp.Event = fn(*resp.Event)
			}
			// Validate the event conforms to the CloudEvents Spec.
			if err := resp.Event.Validate(); err != nil {
				return fmt.Errorf("cloudevent validation failed on response event: %v", err)
			}
		}
		return err
	}
	return nil
}

// Blocking Call
func (c *ceClient) StartReceiver(ctx context.Context, fn interface{}) error {
	c.receiverMu.Lock()
	defer c.receiverMu.Unlock()

	if c.transport == nil {
		return fmt.Errorf("client not ready, transport not initialized")
	}
	if c.fn != nil {
		return fmt.Errorf("client already has a receiver")
	}

	if fn, err := receiver(fn); err != nil {
		return err
	} else {
		c.fn = fn
	}

	defer func() {
		c.fn = nil
	}()

	return c.transport.StartReceiver(ctx)
}

func (c *ceClient) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}
