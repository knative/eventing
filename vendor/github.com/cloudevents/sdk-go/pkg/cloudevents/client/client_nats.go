package client

import (
	"context"
	"fmt"
	cloudeventsnats "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/nats"
	"github.com/nats-io/go-nats"
	"log"
)

func NewNATSClient(natsServer, subject string, opts ...Option) (Client, error) {
	conn, err := nats.Connect(natsServer)
	if err != nil {
		return nil, err
	}
	transport := cloudeventsnats.Transport{
		Conn:    conn,
		Subject: subject,
	}
	c := &ceClient{
		transport: &transport,
	}

	if err := c.applyOptions(opts...); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *ceClient) startNATSReceiver(ctx context.Context, t *cloudeventsnats.Transport, fn Receiver) error {
	if t.Conn == nil {
		return fmt.Errorf("nats connection is required to be set")
	}
	if c.receiver != nil {
		return fmt.Errorf("client already has a receiver")
	}
	if t.Receiver != nil {
		return fmt.Errorf("transport already has a receiver")
	}

	c.receiver = fn
	t.Receiver = c

	go func() {
		if err := t.Listen(ctx); err != nil {
			log.Fatalf("failed to listen, %s", err.Error())
		}
	}()

	return nil
}
