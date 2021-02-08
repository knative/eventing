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

package control

import (
	"context"
	"encoding"
)

type OpCode uint8

const AckOpCode OpCode = 0

type ServiceMessage struct {
	inboundMessage *InboundMessage
	ackFunc        func()
}

func NewServiceMessage(inboundMessage *InboundMessage, ackFunc func()) ServiceMessage {
	return ServiceMessage{
		inboundMessage: inboundMessage,
		ackFunc:        ackFunc,
	}
}

func (c ServiceMessage) Headers() MessageHeader {
	return c.inboundMessage.MessageHeader
}

func (c ServiceMessage) Payload() []byte {
	return c.inboundMessage.Payload
}

// Ack this message to the other end of the connection.
func (c ServiceMessage) Ack() {
	c.ackFunc()
}

// MessageHandler is an error handler for ServiceMessage.
// Every implementation should invoke message.Ack() when the Service can ack back to the other end of the connection.
type MessageHandler interface {
	HandleServiceMessage(ctx context.Context, message ServiceMessage)
}

type MessageHandlerFunc func(ctx context.Context, message ServiceMessage)

func (c MessageHandlerFunc) HandleServiceMessage(ctx context.Context, message ServiceMessage) {
	c(ctx, message)
}

// ErrorHandler is an error handler for the Service interface
type ErrorHandler interface {
	HandleServiceError(ctx context.Context, err error)
}

type ErrorHandlerFunc func(ctx context.Context, err error)

func (c ErrorHandlerFunc) HandleServiceError(ctx context.Context, err error) {
	c(ctx, err)
}

// Service is the high level interface that handles send with retries and acks
type Service interface {
	// SendAndWaitForAck sends a message to the other end and waits for the ack
	SendAndWaitForAck(opcode OpCode, payload encoding.BinaryMarshaler) error

	// MessageHandler sets a MessageHandler to this service.
	// This method is non blocking, because a polling loop is already running inside.
	MessageHandler(handler MessageHandler)

	// ErrorHandler sets a ErrorHandler to this service.
	// This method is non blocking, because a polling loop is already running inside.
	ErrorHandler(handler ErrorHandler)
}

// ServiceWrapper wraps a service in another service to offer additional functionality
type ServiceWrapper func(Service) Service
