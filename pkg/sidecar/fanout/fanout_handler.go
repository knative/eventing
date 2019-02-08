/*
Copyright 2018 The Knative Authors

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

// Package fanout provides an http.Handler that takes in one request and fans it out to N other
// requests, based on a list of Subscriptions. Logically, it represents all the Subscriptions to a
// single Knative Channel.
// It will normally be used in conjunction with multichannelfanout.Handler, which contains multiple
// fanout.Handlers, each corresponding to a single Knative Channel.
package fanout

import (
	"errors"
	"net/http"
	"time"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
)

const (
	defaultTimeout = 1 * time.Minute

	messageBufferSize = 500
)

// Config for a fanout.Handler.
type Config struct {
	Subscriptions []eventingduck.ChannelSubscriberSpec `json:"subscriptions"`
	// AsyncHandler controls whether the Subscriptions are called synchronous or asynchronously.
	// It is expected to be false when used as a sidecar.
	AsyncHandler bool `json:"asyncHandler,omitempty"`
}

// Handler is a http.Handler that takes a single request in and fans it out to N other servers.
type Handler struct {
	config Config

	receivedMessages chan *forwardMessage
	receiver         *provisioners.MessageReceiver
	dispatcher       *provisioners.MessageDispatcher

	// TODO: Plumb context through the receiver and dispatcher and use that to store the timeout,
	// rather than a member variable.
	timeout time.Duration

	logger *zap.Logger
}

var _ http.Handler = &Handler{}

// forwardMessage is passed between the Receiver and the Dispatcher.
type forwardMessage struct {
	msg  *provisioners.Message
	done chan<- error
}

// NewHandler creates a new fanout.Handler.
func NewHandler(logger *zap.Logger, config Config) *Handler {
	handler := &Handler{
		logger:           logger,
		config:           config,
		dispatcher:       provisioners.NewMessageDispatcher(logger.Sugar()),
		receivedMessages: make(chan *forwardMessage, messageBufferSize),
		timeout:          defaultTimeout,
	}
	// The receiver function needs to point back at the handler itself, so set it up after
	// initialization.
	handler.receiver = provisioners.NewMessageReceiver(createReceiverFunction(handler), logger.Sugar())

	return handler
}

func createReceiverFunction(f *Handler) func(provisioners.ChannelReference, *provisioners.Message) error {
	return func(_ provisioners.ChannelReference, m *provisioners.Message) error {
		if f.config.AsyncHandler {
			go func() {
				// Any returned error is already logged in f.dispatch().
				_ = f.dispatch(m)
			}()
			return nil
		}
		return f.dispatch(m)
	}
}

func (f *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.receiver.HandleRequest(w, r)
}

// dispatch takes the request, fans it out to each subscription in f.config. If all the fanned out
// requests return successfully, then return nil. Else, return an error.
func (f *Handler) dispatch(msg *provisioners.Message) error {
	errorCh := make(chan error, len(f.config.Subscriptions))
	for _, sub := range f.config.Subscriptions {
		go func(s eventingduck.ChannelSubscriberSpec) {
			errorCh <- f.makeFanoutRequest(*msg, s)
		}(sub)
	}

	for range f.config.Subscriptions {
		select {
		case err := <-errorCh:
			if err != nil {
				f.logger.Error("Fanout had an error", zap.Error(err))
				return err
			}
		case <-time.After(f.timeout):
			f.logger.Error("Fanout timed out")
			return errors.New("fanout timed out")
		}
	}
	// All Subscriptions returned err = nil.
	return nil
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (f *Handler) makeFanoutRequest(m provisioners.Message, sub eventingduck.ChannelSubscriberSpec) error {
	return f.dispatcher.DispatchMessage(&m, sub.SubscriberURI, sub.ReplyURI, provisioners.DispatchDefaults{})
}
