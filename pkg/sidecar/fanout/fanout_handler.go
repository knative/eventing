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

package fanout

import (
	"errors"
	"github.com/knative/eventing/pkg/buses"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"net/http"
	"time"
)

const (
	defaultTimeout = 1 * time.Minute

	messageBufferSize = 500
)

// Configuration for a fanout.Handler.
type Config struct {
	Subscriptions []duckv1alpha1.ChannelSubscriberSpec `json:"subscriptions"`
}

// http.Handler that takes a single request in and fans it out to N other servers.
type Handler struct {
	config Config

	receivedMessages chan *message
	receiver         *buses.MessageReceiver
	dispatcher       *buses.MessageDispatcher
	stopCh chan<- struct{}

	timeout time.Duration

	logger *zap.Logger
}

var _ http.Handler = &Handler{}

// message is passed between the Receiver and the Dispatcher.
type message struct {
	msg *buses.Message
	done chan<- error
}

// NewHandler creates a new fanout.Handler and starts a background goroutine to make it work. The
// caller is responsible for calling handler.Stop(), when the handler is to be stopped.
func NewHandler(logger *zap.Logger, config Config) *Handler {
	stopCh := make(chan struct{}, 1)
	handler := &Handler{
		logger:     logger,
		config:     config,
		dispatcher: buses.NewMessageDispatcher(logger.Sugar()),
		receivedMessages: make(chan *message, messageBufferSize),
		stopCh: stopCh,
		timeout: defaultTimeout,
	}
	// The receiver function needs to point back at the handler itself, so setup it after
	// initialization.
	handler.receiver = buses.NewMessageReceiver(createReceiverFunction(handler), logger.Sugar())

	go handler.foreverDispatch(stopCh)
	return handler
}

func createReceiverFunction(f *Handler) func(buses.ChannelReference, *buses.Message) error {
	return func(_ buses.ChannelReference, m *buses.Message) error {
		done := make(chan error)
		msg := &message{
			msg: m,
			done: done,
		}

		select {
		case f.receivedMessages <- msg:
			// Continue to next select.
		default:
			f.logger.Debug("Unable to add message to channel, dropping it.", zap.Any("msg", msg))
			return errors.New("unable to add message to channel, dropping it")
		}

		select {
		case possibleErr := <-done:
			f.logger.Debug("Responding", zap.Any("possibleErr", possibleErr))
			return possibleErr
		case <-time.After(f.timeout):
			f.logger.Debug("Timeout waiting for dispatch")
			return errors.New("timeout waiting for dispatch")
		}
	}
}

func (f *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.receiver.HandleRequest(w, r)
}

// Stop stops the background goroutine. Once this is called, this Handler will no longer properly
// Serve HTTP traffic.
func (f *Handler) Stop() {
	f.stopCh <- struct{}{}
}

// foreverDispatch dispatches received messages in an infinite loop. It exits only when
// Handler.stopCh receives a message. It is meant to be run as a goroutine.
func (f *Handler) foreverDispatch(stopCh <-chan struct{}) {
	for {
		select {
		case msg := <- f.receivedMessages:
			f.dispatch(msg)
		case <-stopCh:
			f.logger.Info("Fanout dispatch thread stopping.")
			return
		}
	}
}

// ServeHTTP takes the request, fans it out to each subscription in f.config. If all the fanned out
// requests return successfully, then return successfully. Else, return failure.
func (f *Handler) dispatch(msg *message) {
	msg.done <- func() error {
		errorCh := make(chan error, len(f.config.Subscriptions))
		for _, sub := range f.config.Subscriptions {
			go func(s duckv1alpha1.ChannelSubscriberSpec) {
				errorCh <- f.makeFanoutRequest(*msg.msg, s)
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
		// All Subscriptions returned err=nil.
		return nil
	}()
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `to` portions of the subscription.
func (f *Handler) makeFanoutRequest(m buses.Message, sub duckv1alpha1.ChannelSubscriberSpec) error {
	return f.dispatcher.DispatchMessage(&m, sub.CallableDomain, sub.SinkableDomain, buses.DispatchDefaults{})
}
