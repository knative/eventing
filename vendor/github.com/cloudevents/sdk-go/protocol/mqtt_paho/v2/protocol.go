/*
 Copyright 2023 The CloudEvents Authors
 SPDX-License-Identifier: Apache-2.0
*/

package mqtt_paho

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol"
	"github.com/eclipse/paho.golang/paho"

	cecontext "github.com/cloudevents/sdk-go/v2/context"
)

type Protocol struct {
	client          *paho.Client
	config          *paho.ClientConfig
	connOption      *paho.Connect
	publishOption   *paho.Publish
	subscribeOption *paho.Subscribe

	// receiver
	incoming chan *paho.Publish
	// inOpen
	openerMutex sync.Mutex

	closeChan chan struct{}
}

var (
	_ protocol.Sender   = (*Protocol)(nil)
	_ protocol.Opener   = (*Protocol)(nil)
	_ protocol.Receiver = (*Protocol)(nil)
	_ protocol.Closer   = (*Protocol)(nil)
)

func New(ctx context.Context, config *paho.ClientConfig, opts ...Option) (*Protocol, error) {
	if config == nil {
		return nil, fmt.Errorf("the paho.ClientConfig must not be nil")
	}

	p := &Protocol{
		client: paho.NewClient(*config),
		// default connect option
		connOption: &paho.Connect{
			KeepAlive:  30,
			CleanStart: true,
		},
		incoming:  make(chan *paho.Publish),
		closeChan: make(chan struct{}),
	}
	if err := p.applyOptions(opts...); err != nil {
		return nil, err
	}

	// Connect to the MQTT broker
	connAck, err := p.client.Connect(ctx, p.connOption)
	if err != nil {
		return nil, err
	}
	if connAck.ReasonCode != 0 {
		return nil, fmt.Errorf("failed to connect to %q : %d - %q", p.client.Conn.RemoteAddr(), connAck.ReasonCode,
			connAck.Properties.ReasonString)
	}

	return p, nil
}

func (p *Protocol) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(p); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) Send(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
	if p.publishOption == nil {
		return fmt.Errorf("the paho.Publish option must not be nil")
	}

	var err error
	defer m.Finish(err)

	msg := p.publishOption
	if cecontext.TopicFrom(ctx) != "" {
		msg.Topic = cecontext.TopicFrom(ctx)
		cecontext.WithTopic(ctx, "")
	}

	err = WritePubMessage(ctx, m, msg, transformers...)
	if err != nil {
		return err
	}

	_, err = p.client.Publish(ctx, msg)
	if err != nil {
		return err
	}
	return err
}

func (p *Protocol) OpenInbound(ctx context.Context) error {
	if p.subscribeOption == nil {
		return fmt.Errorf("the paho.Subscribe option must not be nil")
	}

	p.openerMutex.Lock()
	defer p.openerMutex.Unlock()

	logger := cecontext.LoggerFrom(ctx)

	p.client.Router = paho.NewSingleHandlerRouter(func(m *paho.Publish) {
		p.incoming <- m
	})

	logger.Infof("subscribing to topics: %v", p.subscribeOption.Subscriptions)
	_, err := p.client.Subscribe(ctx, p.subscribeOption)
	if err != nil {
		return err
	}

	// Wait until external or internal context done
	select {
	case <-ctx.Done():
	case <-p.closeChan:
	}
	return p.client.Disconnect(&paho.Disconnect{ReasonCode: 0})
}

// Receive implements Receiver.Receive
func (p *Protocol) Receive(ctx context.Context) (binding.Message, error) {
	select {
	case m, ok := <-p.incoming:
		if !ok {
			return nil, io.EOF
		}
		msg := NewMessage(m)
		return msg, nil
	case <-ctx.Done():
		return nil, io.EOF
	}
}

func (p *Protocol) Close(ctx context.Context) error {
	close(p.closeChan)
	return nil
}
