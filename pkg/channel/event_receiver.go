/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package channel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/eventing/pkg/utils"
)

var (
	shutdownTimeout = 1 * time.Minute
)

// UnknownChannelError represents the error when an event is received by a channel dispatcher for a
// channel that does not exist.
type UnknownChannelError struct {
	c ChannelReference
}

func (e *UnknownChannelError) Error() string {
	return fmt.Sprintf("unknown channel: %v", e.c)
}

// EventReceiver starts a server to receive new events for the channel dispatcher. The new
// event is emitted via the receiver function.
type EventReceiver struct {
	ceClient          cloudevents.Client
	receiverFunc      ReceiverFunc
	logger            *zap.Logger
	hostToChannelFunc ResolveChannelFromHostFunc
}

// ReceiverFunc is the function to be called for handling the event.
type ReceiverFunc func(context.Context, ChannelReference, cloudevents.Event) error

// ReceiverOptions provides functional options to EventReceiver function.
type ReceiverOptions func(*EventReceiver) error

// ResolveChannelFromHostFunc function enables EventReceiver to get the Channel Reference from incoming request HostHeader
// before calling receiverFunc.
type ResolveChannelFromHostFunc func(string) (ChannelReference, error)

// ResolveChannelFromHostHeader is a ReceiverOption for NewEventReceiver which enables the caller to overwrite the
// default behaviour defined by ParseChannel function.
func ResolveChannelFromHostHeader(hostToChannelFunc ResolveChannelFromHostFunc) ReceiverOptions {
	return func(r *EventReceiver) error {
		r.hostToChannelFunc = hostToChannelFunc
		return nil
	}
}

// NewEventReceiver creates an event receiver passing new events to the
// receiverFunc.
func NewEventReceiver(receiverFunc ReceiverFunc, logger *zap.Logger, opts ...ReceiverOptions) (*EventReceiver, error) {
	ceClient, err := kncloudevents.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create cloudevents client: %v", err)
	}
	receiver := &EventReceiver{
		ceClient:          ceClient,
		receiverFunc:      receiverFunc,
		hostToChannelFunc: ResolveChannelFromHostFunc(ParseChannel),
		logger:            logger,
	}
	for _, opt := range opts {
		if err := opt(receiver); err != nil {
			return nil, err
		}
	}
	return receiver, nil
}

// Start begins to receive events for the receiver.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
func (r *EventReceiver) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- r.ceClient.StartReceiver(ctx, r.receiverFunc)
	}()

	// Stop either if the receiver stops (sending to errCh) or if the context Done channel is closed.
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		break
	}

	// Done channel has been closed, we need to gracefully shutdown r.ceClient. The cancel() method will start its
	// shutdown, if it hasn't finished in a reasonable amount of time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(shutdownTimeout):
		return errors.New("timeout shutting down ceClient")
	}
}

func (r *EventReceiver) ServeHTTP(ctx context.Context, event cloudevents.Event, resp *cloudevents.EventResponse) error {
	tctx := cloudevents.HTTPTransportContextFrom(ctx)
	if tctx.Method != http.MethodPost {
		resp.Status = http.StatusMethodNotAllowed
		return nil
	}

	// tctx.URI is actually the path...
	if tctx.URI != "/" {
		resp.Status = http.StatusNotFound
		return nil
	}

	// The response status codes:
	//   202 - the event was sent to subscribers
	//   404 - the request was for an unknown channel
	//   500 - an error occurred processing the request

	host := tctx.Host
	r.logger.Debug("Received request", zap.String("host", host))
	channel, err := r.hostToChannelFunc(host)
	if err != nil {
		r.logger.Info("Could not extract channel", zap.Error(err))
		resp.Status = http.StatusInternalServerError
		return err
	}
	r.logger.Debug("Request mapped to channel", zap.String("channel", channel.String()))

	sctx := utils.ReceivingContextFrom(ctx)
	AppendHistory(&event, host)

	event = tracing.AddTraceparentAttributeFromContext(ctx, event)
	err = r.receiverFunc(sctx, channel, event)
	if err != nil {
		if _, ok := err.(*UnknownChannelError); ok {
			resp.Status = http.StatusNotFound
		} else {
			resp.Status = http.StatusInternalServerError
		}
		return err
	}

	resp.Status = http.StatusAccepted
	return nil
}

// ParseChannel converts the channel's hostname into a channel
// reference.
func ParseChannel(host string) (ChannelReference, error) {
	chunks := strings.Split(host, ".")
	if len(chunks) < 2 {
		return ChannelReference{}, fmt.Errorf("bad host format %q", host)
	}
	return ChannelReference{
		Name:      chunks[0],
		Namespace: chunks[1],
	}, nil
}
