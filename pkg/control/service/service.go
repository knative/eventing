/*
Copyright 2021 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"context"
	"encoding"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"knative.dev/pkg/logging"

	ctrl "knative.dev/eventing/pkg/control"
)

const (
	controlServiceSendTimeout = 15 * time.Second
)

type service struct {
	ctx context.Context

	connection ctrl.Connection

	waitingAcksMutex sync.Mutex
	waitingAcks      map[uuid.UUID]chan interface{}

	handlerMutex sync.RWMutex
	handler      ctrl.MessageHandler

	errorHandlerMutex sync.RWMutex
	errorHandler      ctrl.ErrorHandler
}

func NewService(ctx context.Context, connection ctrl.Connection) *service {
	cs := &service{
		ctx:          ctx,
		connection:   connection,
		waitingAcks:  make(map[uuid.UUID]chan interface{}),
		handler:      NoopMessageHandler,
		errorHandler: LoggerErrorHandler,
	}
	cs.startPolling()
	return cs
}

func (c *service) SendAndWaitForAck(opcode ctrl.OpCode, payload encoding.BinaryMarshaler) error {
	b, err := payload.MarshalBinary()
	if err != nil {
		return err
	}
	return c.sendBinaryAndWaitForAck(opcode, b)
}

func (c *service) sendBinaryAndWaitForAck(opcode ctrl.OpCode, payload []byte) error {
	if opcode == ctrl.AckOpCode {
		return fmt.Errorf("you cannot send an ack manually")
	}
	msg := ctrl.NewOutboundMessage(uint8(opcode), payload)

	logging.FromContext(c.ctx).Debugf("Going to send message with opcode %d and uuid %s", msg.OpCode(), msg.UUID().String())

	// Register the ack between the waiting acks
	ackCh := make(chan interface{}, 1)
	c.waitingAcksMutex.Lock()
	c.waitingAcks[msg.UUID()] = ackCh
	c.waitingAcksMutex.Unlock()

	defer func() {
		c.waitingAcksMutex.Lock()
		delete(c.waitingAcks, msg.UUID())
		c.waitingAcksMutex.Unlock()
	}()

	c.connection.OutboundMessages() <- &msg

	select {
	case <-ackCh:
		return nil
	case <-c.ctx.Done():
		logging.FromContext(c.ctx).Warnf("Dropping message because context cancelled: %s", msg.UUID().String())
		return c.ctx.Err()
	case <-time.After(controlServiceSendTimeout):
		logging.FromContext(c.ctx).Debugf("Timeout waiting for the ack, retrying to send: %s", msg.UUID().String())
		return fmt.Errorf("retry exceeded for outgoing message: %s", msg.UUID().String())
	}
}

func (c *service) MessageHandler(handler ctrl.MessageHandler) {
	c.handlerMutex.Lock()
	c.handler = handler
	c.handlerMutex.Unlock()
}

func (c *service) ErrorHandler(handler ctrl.ErrorHandler) {
	c.errorHandlerMutex.Lock()
	c.errorHandler = handler
	c.errorHandlerMutex.Unlock()
}

func (c *service) startPolling() {
	go func() {
		for {
			select {
			case msg, ok := <-c.connection.InboundMessages():
				if !ok {
					logging.FromContext(c.ctx).Debugf("InboundMessages channel closed, closing the polling")
					return
				}
				go c.accept(msg)
			case err, ok := <-c.connection.Errors():
				if !ok {
					logging.FromContext(c.ctx).Debugf("Errors channel closed")
				}
				go c.acceptError(err)
			case <-c.ctx.Done():
				logging.FromContext(c.ctx).Debugf("Context closed, closing polling loop of control service")
				return
			}
		}
	}()
}

func (c *service) accept(msg *ctrl.InboundMessage) {
	if msg.OpCode() == uint8(ctrl.AckOpCode) {
		// Propagate the ack
		c.waitingAcksMutex.Lock()
		ackCh := c.waitingAcks[msg.UUID()]
		c.waitingAcksMutex.Unlock()
		if ackCh != nil {
			close(ackCh)
			logging.FromContext(c.ctx).Debugf("Acked message: %s", msg.UUID().String())
		} else {
			logging.FromContext(c.ctx).Debugf("Ack received but no channel available: %s", msg.UUID().String())
		}
	} else {
		ackFunc := func() {
			ackMsg := newAckMessage(msg.UUID())
			// Before resending, check if context is not closed
			select {
			case <-c.ctx.Done():
				return
			default:
				c.connection.OutboundMessages() <- &ackMsg
			}
		}
		c.handlerMutex.RLock()
		c.handler.HandleServiceMessage(c.ctx, ctrl.NewServiceMessage(msg, ackFunc))
		c.handlerMutex.RUnlock()
	}
}

func (c *service) acceptError(err error) {
	c.errorHandlerMutex.RLock()
	c.errorHandler.HandleServiceError(c.ctx, err)
	c.errorHandlerMutex.RUnlock()
}

func newAckMessage(uuid [16]byte) ctrl.OutboundMessage {
	return ctrl.NewOutboundMessageWithUUID(uuid, uint8(ctrl.AckOpCode), nil)
}
