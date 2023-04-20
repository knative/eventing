/*
Copyright 2020 The Knative Authors

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

package channel

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/buffering"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"

	"knative.dev/pkg/network"

	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/utils"
)

// UnknownChannelError represents the error when an event is received by a channel dispatcher for a
// channel that does not exist.
type UnknownChannelError struct {
	Channel ChannelReference
}

func (e *UnknownChannelError) Error() string {
	return fmt.Sprint("unknown channel: ", e.Channel)
}

// UnknownHostError represents the error when a ResolveMessageChannelFromHostHeader func cannot resolve an host
type UnknownHostError string

func (e UnknownHostError) Error() string {
	return "cannot map host to channel: " + string(e)
}

type BadRequestError string

func (e BadRequestError) Error() string {
	return "malformed request: " + string(e)
}

// MessageReceiver starts a server to receive new events for the channel dispatcher. The new
// event is emitted via the receiver function.
type MessageReceiver struct {
	httpBindingsReceiver *kncloudevents.HTTPMessageReceiver
	receiverFunc         UnbufferedMessageReceiverFunc
	logger               *zap.Logger
	hostToChannelFunc    ResolveChannelFromHostFunc
	pathToChannelFunc    ResolveChannelFromPathFunc
	reporter             StatsReporter
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
// default behaviour defined by ParseChannelFromHost function.
func ResolveMessageChannelFromHostHeader(hostToChannelFunc ResolveChannelFromHostFunc) MessageReceiverOptions {
	return func(r *MessageReceiver) error {
		r.hostToChannelFunc = hostToChannelFunc
		return nil
	}
}

// ResolveChannelFromPathFunc function enables EventReceiver to get the Channel Reference from incoming request's path
// before calling receiverFunc.
type ResolveChannelFromPathFunc func(string) (ChannelReference, error)

// ResolveMessageChannelFromPath is a ReceiverOption for NewMessageReceiver which enables the caller to overwrite the
// default behaviour defined by ParseChannelFromPath function.
func ResolveMessageChannelFromPath(PathToChannelFunc ResolveChannelFromPathFunc) MessageReceiverOptions {
	return func(r *MessageReceiver) error {
		r.pathToChannelFunc = PathToChannelFunc
		return nil
	}
}

// NewMessageReceiver creates an event receiver passing new events to the
// receiverFunc.
func NewMessageReceiver(receiverFunc UnbufferedMessageReceiverFunc, logger *zap.Logger, reporter StatsReporter, opts ...MessageReceiverOptions) (*MessageReceiver, error) {
	bindingsReceiver := kncloudevents.NewHTTPMessageReceiver(8080)
	receiver := &MessageReceiver{
		httpBindingsReceiver: bindingsReceiver,
		receiverFunc:         receiverFunc,
		hostToChannelFunc:    ResolveChannelFromHostFunc(ParseChannelFromHost),
		logger:               logger,
		reporter:             reporter,
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
	// shutdown, if it hasn't finished in a reasonable amount of Time, just return an error.
	cancel()
	select {
	case err := <-errCh:
		return err
	case <-time.After(network.DefaultDrainTimeout):
		return errors.New("timeout shutting down http bindings receiver")
	}
}

func (r *MessageReceiver) ServeHTTP(response nethttp.ResponseWriter, request *nethttp.Request) {
	response.Header().Set("Allow", "POST, OPTIONS")
	if request.Method == nethttp.MethodOptions {
		response.Header().Set("WebHook-Allowed-Origin", "*") // Accept from any Origin:
		response.Header().Set("WebHook-Allowed-Rate", "*")   // Unlimited requests/minute
		response.WriteHeader(nethttp.StatusOK)
		return
	}
	if request.Method != nethttp.MethodPost {
		response.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}

	// The response status codes:
	//   202 - the event was sent to subscribers
	//   400 - the request was malformed
	//   404 - the request was for an unknown channel
	//   500 - an error occurred processing the request
	args := ReportArgs{}
	var channel ChannelReference
	var err error

	// prefer using pathToChannelFunc if available
	if r.pathToChannelFunc != nil {
		channel, err = r.pathToChannelFunc(request.URL.Path)
	} else {
		if request.URL.Path != "/" {
			response.WriteHeader(nethttp.StatusBadRequest)
			return
		}
		channel, err = r.hostToChannelFunc(request.Host)
	}

	if err != nil {
		switch err.(type) {
		case UnknownHostError:
			response.WriteHeader(nethttp.StatusNotFound)
		case BadRequestError:
			response.WriteHeader(nethttp.StatusBadRequest)
		default:
			response.WriteHeader(nethttp.StatusInternalServerError)
		}

		r.logger.Info("Could not extract channel", zap.Error(err))
		ReportEventCountMetricsForDispatchError(err, r.reporter, &args)
		return
	}
	r.logger.Debug("Request mapped to channel", zap.String("channel", channel.String()))

	args.Ns = channel.Namespace

	message := http.NewMessageFromHttpRequest(request)
	if message.ReadEncoding() == binding.EncodingUnknown {
		r.logger.Info("Cannot determine the cloudevent message encoding")
		response.WriteHeader(nethttp.StatusBadRequest)
		r.reporter.ReportEventCount(&args, nethttp.StatusBadRequest)
		return
	}

	bufferedMessage, err := buffering.CopyMessage(request.Context(), message)
	if err != nil {
		r.logger.Warn("Cannot buffer cloudevent message", zap.Error(err))
		response.WriteHeader(nethttp.StatusBadRequest)
		_ = r.reporter.ReportEventCount(&args, nethttp.StatusBadRequest)
		return
	}

	event, err := binding.ToEvent(request.Context(), bufferedMessage)
	if err != nil {
		r.logger.Warn("failed to extract event from request", zap.Error(err))
		response.WriteHeader(nethttp.StatusBadRequest)
		_ = r.reporter.ReportEventCount(&args, nethttp.StatusBadRequest)
		return
	}

	// run validation for the extracted event
	if err := event.Validate(); err != nil {
		r.logger.Warn("failed to validate extracted event", zap.Error(err))
		response.WriteHeader(nethttp.StatusBadRequest)
		return
	}

	err = r.receiverFunc(request.Context(), channel, bufferedMessage, []binding.Transformer{}, utils.PassThroughHeaders(request.Header))
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

func ReportEventCountMetricsForDispatchError(err error, reporter StatsReporter, args *ReportArgs) {
	switch err.(type) {
	case *UnknownChannelError:
		_ = reporter.ReportEventCount(args, nethttp.StatusNotFound)
	case BadRequestError:
		_ = reporter.ReportEventCount(args, nethttp.StatusBadRequest)
	default:
		_ = reporter.ReportEventCount(args, nethttp.StatusInternalServerError)
	}
}

var _ nethttp.Handler = (*MessageReceiver)(nil)
