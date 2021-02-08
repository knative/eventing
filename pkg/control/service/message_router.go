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

	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/control"
)

type MessageRouter map[control.OpCode]control.MessageHandler

func (c MessageRouter) HandleServiceMessage(ctx context.Context, message control.ServiceMessage) {
	logger := logging.FromContext(ctx)

	handler, ok := c[control.OpCode(message.Headers().OpCode())]
	if ok {
		handler.HandleServiceMessage(ctx, message)
		return
	}

	message.Ack()
	logger.Warnw(
		"Received an unknown message, I don't know what to do with it",
		zap.Uint8("opcode", message.Headers().OpCode()),
		zap.ByteString("payload", message.Payload()),
	)
}
