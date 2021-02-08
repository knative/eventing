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

package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	"knative.dev/eventing/pkg/control"
	"knative.dev/eventing/pkg/control/service"
)

func setupConnection(t *testing.T) (control.Service, control.Service) {
	logger, _ := zap.NewDevelopment()
	ctx := logging.WithLogger(context.TODO(), logger.Sugar())

	dataPlane, connectionPool := setupInsecureServerAndConnectionPool(t, ctx)

	clientCtx, clientCancelFn := context.WithCancel(ctx)

	conns, err := connectionPool.ReconcileConnections(clientCtx, "hello", []string{"127.0.0.1"}, nil, nil)
	require.NoError(t, err)
	require.Contains(t, conns, "127.0.0.1")
	t.Cleanup(clientCancelFn)

	controlPlane := conns["127.0.0.1"]

	return controlPlane, dataPlane
}

func TestMessageRouter(t *testing.T) {
	controlPlane, dataPlane := setupConnection(t)

	opcode1Count := atomic.NewInt32(0)
	opcode2Count := atomic.NewInt32(0)

	dataPlane.MessageHandler(service.MessageRouter{
		1: control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
			require.Equal(t, uint8(1), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode1Count.Inc()
		}),
		2: control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
			require.Equal(t, uint8(2), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode2Count.Inc()
		}),
	})

	for i := 0; i < 10; i++ {
		require.NoError(t, controlPlane.SendAndWaitForAck(control.OpCode((i%2)+1), mockMessage("Funky!")))
	}

	require.Equal(t, int32(5), opcode1Count.Load())
	require.Equal(t, int32(5), opcode2Count.Load())
}

func TestMessageRouter_MessageNotMatchingAck(t *testing.T) {
	controlPlane, dataPlane := setupConnection(t)

	opcode1Count := atomic.NewInt32(0)
	opcode2Count := atomic.NewInt32(0)

	dataPlane.MessageHandler(service.MessageRouter{
		1: control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
			require.Equal(t, uint8(1), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode1Count.Inc()
		}),
		2: control.MessageHandlerFunc(func(ctx context.Context, message control.ServiceMessage) {
			require.Equal(t, uint8(2), message.Headers().OpCode())
			require.Equal(t, "Funky!", string(message.Payload()))
			message.Ack()
			opcode2Count.Inc()
		}),
	})

	require.NoError(t, controlPlane.SendAndWaitForAck(10, mockMessage("Funky!")))

	require.Equal(t, int32(0), opcode1Count.Load())
	require.Equal(t, int32(0), opcode2Count.Load())
}
