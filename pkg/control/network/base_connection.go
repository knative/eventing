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

package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"sync"

	"go.uber.org/zap"

	ctrl "knative.dev/eventing/pkg/control"
)

type baseTcpConnection struct {
	ctx    context.Context
	logger *zap.SugaredLogger

	conn      net.Conn
	connMutex sync.RWMutex

	outboundMessageChannel chan *ctrl.OutboundMessage
	inboundMessageChannel  chan *ctrl.InboundMessage
	errors                 chan error
}

func (t *baseTcpConnection) OutboundMessages() chan<- *ctrl.OutboundMessage {
	return t.outboundMessageChannel
}

func (t *baseTcpConnection) InboundMessages() <-chan *ctrl.InboundMessage {
	return t.inboundMessageChannel
}

func (t *baseTcpConnection) Errors() <-chan error {
	return t.errors
}

func (t *baseTcpConnection) read() error {
	msg := &ctrl.InboundMessage{}
	t.connMutex.RLock()
	n, err := msg.ReadFrom(t.conn)
	t.connMutex.RUnlock()
	if err != nil {
		return err
	}
	if n != int64(msg.Length())+24 {
		return fmt.Errorf("the number of read bytes doesn't match the expected length: %d != %d", n, int64(msg.Length())+24)
	}

	t.inboundMessageChannel <- msg
	return nil
}

func (t *baseTcpConnection) write(msg *ctrl.OutboundMessage) error {
	t.connMutex.RLock()
	n, err := msg.WriteTo(t.conn)
	t.connMutex.RUnlock()
	if err != nil {
		return err
	}
	if n != int64(msg.Length())+24 {
		return fmt.Errorf("the number of read bytes doesn't match the expected length: %d != %d", n, int64(msg.Length())+24)
	}
	return nil
}

func (t *baseTcpConnection) consumeConnection(conn net.Conn) {
	t.logger.Infof("Setting new conn: %s", conn.RemoteAddr())
	t.connMutex.Lock()
	t.conn = conn
	t.connMutex.Unlock()

	closedConnCtx, closedConnCancel := context.WithCancel(t.ctx)

	var wg sync.WaitGroup
	wg.Add(2)

	// We have 2 polling loops:
	// * One polls outbound messages and writes to the conn
	// * One reads from the conn and push to inbound messages
	go func() {
		defer wg.Done()
		for {
			select {
			case msg, ok := <-t.outboundMessageChannel:
				if !ok {
					t.logger.Debugf("Outbound channel closed, closing the polling")
					return
				}
				err := t.write(msg)
				if err != nil {
					if isEOF(err) {
						return // Closed conn
					}
					t.tryPropagateError(closedConnCtx, err)
					if !isTransientError(err) {
						return // Broken conn
					}

					// Try to send to outboundMessageChannel if context not closed
					t.tryPushOutboundChannel(closedConnCtx, msg)
				}
			case <-closedConnCtx.Done():
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		defer closedConnCancel()
		for {
			// Blocking read
			err := t.read()
			if err != nil {
				if isEOF(err) {
					return // Closed conn
				}
				t.tryPropagateError(closedConnCtx, err)
				if !isTransientError(err) {
					return // Broken conn
				}
			}

			select {
			case <-closedConnCtx.Done():
				return
			default:
				continue
			}
		}
	}()

	wg.Wait()

	t.logger.Debugf("Stopped consuming connection with local %s and remote %s", conn.LocalAddr().String(), conn.RemoteAddr().String())
	t.connMutex.RLock()
	err := t.conn.Close()
	t.connMutex.RUnlock()
	if err != nil && !isEOF(err) && !isUseOfClosedConnection(err) {
		t.logger.Warnf("Error while closing the previous connection: %s", err)
	}
}

func (t *baseTcpConnection) tryPropagateError(ctx context.Context, err error) {
	select {
	case <-ctx.Done():
		return
	default:
		t.errors <- err
	}
}

func (t *baseTcpConnection) tryPushOutboundChannel(ctx context.Context, msg *ctrl.OutboundMessage) {
	select {
	case <-ctx.Done():
		return
	default:
		t.outboundMessageChannel <- msg
	}
}

func (t *baseTcpConnection) close() (err error) {
	t.connMutex.RLock()
	if t.conn != nil {
		err = t.conn.Close()
	}
	t.connMutex.RUnlock()
	close(t.inboundMessageChannel)
	close(t.outboundMessageChannel)
	close(t.errors)
	if err == nil || isEOF(err) || isUseOfClosedConnection(err) {
		return nil
	}
	return err
}

func isTransientError(err error) bool {
	// Transient errors are fine
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() || neterr.Timeout() {
			return true
		}
	}
	return false
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

var isUseOfClosedConnectionRegex = regexp.MustCompile("use of closed.* connection")

func isUseOfClosedConnection(err error) bool {
	// Don't rely on this check, it's just used to reduce logging noise, it shouldn't be used as assertion
	return isUseOfClosedConnectionRegex.MatchString(err.Error())
}
