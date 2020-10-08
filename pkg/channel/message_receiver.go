/*
 * Copyright 2020 The Knative Authors
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
	nethttp "net/http"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"

	"knative.dev/pkg/network"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
)

// UnknownChannelError represents the error when an event is received by a channel dispatcher for a
// channel that does not exist.
type UnknownChannelError struct {
	c ChannelReference
}

func (e *UnknownChannelError) Error() string {
	return fmt.Sprint("unknown channel: ", e.c)
}

// UnknownHostError represents the error when a ResolveMessageChannelFromHostHeader func cannot resolve an host
type UnknownHostError string

func (e UnknownHostError) Error() string {
	return "cannot map host to channel: " + string(e)
}

// MessageReceiver starts a server to receive new events for the channel dispatcher. The new
// event is emitted via the receiver function.
type MessageReceiver struct {
	httpBindingsReceiver *kncloudevents.HTTPMessageReceiver
	receiverFunc         UnbufferedMessageReceiverFunc
	logger               *zap.Logger
	hostToChannelFunc    ResolveChannelFromHostFunc
}

// UnbufferedMessageReceiverFunc is the function to be called for handling the message.
// The provided message is not buffered, so it can't be safely read more times.
// When you perform the write (or the buffering) of the Message, you must use the transformers provided as parameters.
// This function is responsible for invoking Message.Finish().
type UnbufferedMessageReceiverFunc func(context.Context, ChannelReference, binding.Message, []binding.Transformer, nethttp.Header) error

// ReceiverOptions provides functional options to MessageReceiver function.
type MessageReceiverOptions func(*MessageReceiver) error

// ResolveChannelFromHostFunc function enables EventReceiver to get the Channel Reference from incoming request HostHeader
// before calling receiverFunc.
// Returns UnknownHostError if the channel is not found, otherwise returns a generic error.
type ResolveChannelFromHostFunc func(string) (ChannelReference, error)

// ResolveMessageChannelFromHostHeader is a ReceiverOption for NewMessageReceiver which enables the caller to overwrite the
// default behaviour defined by ParseChannel function.
func ResolveMessageChannelFromHostHeader(hostToChannelFunc ResolveChannelFromHostFunc) MessageReceiverOptions {
	return func(r *MessageReceiver) error {
		r.hostToChannelFunc = hostToChannelFunc
		return nil
	}
}

// NewMessageReceiver creates an event receiver passing new events to the
// receiverFunc.
func NewMessageReceiver(receiverFunc UnbufferedMessageReceiverFunc, logger *zap.Logger, opts ...MessageReceiverOptions) (*MessageReceiver, error) {
	bindingsReceiver := kncloudevents.NewHTTPMessageReceiver(8080)
	receiver := &MessageReceiver{
		httpBindingsReceiver: bindingsReceiver,
		receiverFunc:         receiverFunc,
		hostToChannelFunc:    ResolveChannelFromHostFunc(ParseChannel),
		logger:               logger,
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
func (r *MessageReceiver) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- r.httpBindingsReceiver.StartListen(ctx, r)
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
	case <-time.After(network.DefaultDrainTimeout):
		return errors.New("timeout shutting down http bindings receiver")
	}
}

func (r *MessageReceiver) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	// tctx.URI is actually the path...
	if request.URL.Path != "/" {
		response.WriteHeader(nethttp.StatusNotFound)
		return
	}

	// The response status codes:
	//   202 - the event was sent to subscribers
	//   404 - the request was for an unknown channel
	//   500 - an error occurred processing the request
	host := request.Host
	r.logger.Debug("Received request", zap.String("host", host))
	channel, err := r.hostToChannelFunc(host)
	if err != nil {
		if _, ok := err.(UnknownHostError); ok {
			response.WriteHeader(nethttp.StatusNotFound)
			r.logger.Info(err.Error())
		} else {
			r.logger.Info("Could not extract channel", zap.Error(err))
			response.WriteHeader(nethttp.StatusInternalServerError)
		}
		return
	}
	r.logger.Debug("Request mapped to channel", zap.String("channel", channel.String()))

	message := http.NewMessageFromHttpRequest(request)

	if message.ReadEncoding() == binding.EncodingUnknown {
		r.logger.Info("Cannot determine the cloudevent message encoding")
		response.WriteHeader(nethttp.StatusBadRequest)
		return
	}

	err = r.receiverFunc(request.Context(), channel, message, []binding.Transformer{AddHistory(host)}, utils.PassThroughHeaders(request.Header))
	if err != nil {
		if _, ok := err.(*UnknownChannelError); ok {
			response.WriteHeader(nethttp.StatusNotFound)
		} else {
			r.logger.Info("Error in receiver", zap.Error(err))
			response.WriteHeader(nethttp.StatusInternalServerError)
		}
		return
	}

	response.WriteHeader(nethttp.StatusAccepted)
}

var _ nethttp.Handler = (*MessageReceiver)(nil)
