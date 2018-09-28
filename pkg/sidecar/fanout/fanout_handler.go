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
	"fmt"
	"github.com/knative/eventing/pkg/buses"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	// The header attached to requests sent through the MessageReceiver. It is used to correlate the
	// request going into the MessageReceiver and the request coming out of the MessageReceiver. It
	// is removed before being sent downstream.
	// Note that it MUST be in the headers forwarded by the MessageReceiver.
	uniqueFanoutHeader = "Knative-Fanout-Message-Tracker"

	defaultTimeout = 1 * time.Minute
)

// Configuration for a fanout.Handler.
type Config struct {
	Subscriptions []duckv1alpha1.ChannelSubscriberSpec `json:"subscriptions"`
}

// http.Handler that takes a single request in and fans it out to N other servers.
type fanoutHandler struct {
	config Config

	receivedMessages *messageStorage
	receiver         *buses.MessageReceiver
	dispatcher       *buses.MessageDispatcher

	timeout time.Duration

	logger *zap.Logger
}

var _ http.Handler = &fanoutHandler{}

func NewHandler(logger *zap.Logger, config Config) http.Handler {
	handler := &fanoutHandler{
		logger:     logger,
		config:     config,
		dispatcher: buses.NewMessageDispatcher(logger.Sugar()),
		receivedMessages: &messageStorage{
			messages: make(map[string]*buses.Message),
		},
		timeout: defaultTimeout,
	}
	// The receiver function needs to point back at the handler itself, so setup it after
	// initialization.
	handler.receiver = buses.NewMessageReceiver(createReceiverFunction(handler), logger.Sugar())
	return handler
}

func createReceiverFunction(f *fanoutHandler) func(buses.ChannelReference, *buses.Message) error {
	return func(_ buses.ChannelReference, m *buses.Message) error {
		f.logger.Debug("Putting message", zap.String("key", m.Headers[uniqueFanoutHeader]))
		f.receivedMessages.Put(m)
		return nil
	}
}

// ServeHTTP takes the request, fans it out to each subscription in f.config. If all the fanned out
// requests return successfully, then return successfully. Else, return failure.
func (f *fanoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := addTrackingHeader(r)
	receiverResponse := &response{}
	f.receiver.HandleRequest(receiverResponse, r)

	if receiverResponse.statusCode != http.StatusAccepted {
		f.logger.Info("MessageReceiver rejected the request", zap.Int("statusCode", receiverResponse.statusCode))
		w.WriteHeader(receiverResponse.statusCode)
		// We don't care about the message, we just don't want it to stay in the map forever.
		f.receivedMessages.pull(key)
		return
	}

	f.logger.Debug("Pulling message", zap.String("key", key))
	m, err := f.receivedMessages.pull(key)
	if err != nil {
		f.logger.Info("Could not find tracked message", zap.Error(err), zap.Any("key", r.Header[uniqueFanoutHeader]))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	removeTrackingHeader(m)

	errorCh := make(chan error, len(f.config.Subscriptions))
	for _, sub := range f.config.Subscriptions {
		go func(s duckv1alpha1.ChannelSubscriberSpec) {
			errorCh <- f.makeFanoutRequest(r, *m, s)
		}(sub)
	}

	sc := http.StatusOK
Loop:
	for range f.config.Subscriptions {
		select {
		case err := <-errorCh:
			if err != nil {
				f.logger.Error("Fanout had an error", zap.Error(err))
				sc = http.StatusInternalServerError
			}
		case <-time.After(f.timeout):
			f.logger.Error("Fanout timed out")
			sc = http.StatusInternalServerError
			break Loop
		}
	}

	w.WriteHeader(sc)
}

// makeFanoutRequest sends the request to exactly one subscription. It handles both the `call` and
// the `to` portions of the subscription.
func (f *fanoutHandler) makeFanoutRequest(r *http.Request, m buses.Message, sub duckv1alpha1.ChannelSubscriberSpec) error {
	return f.dispatcher.DispatchMessage(&m, sub.CallableDomain, sub.SinkableDomain, buses.DispatchDefaults{})
}

// response is used to capture the statusCode returned by the messageReceiver.
type response struct {
	statusCode int
}

var _ http.ResponseWriter = &response{}

func (*response) Header() http.Header {
	return http.Header{}
}

func (*response) Write(b []byte) (int, error) {
	return len(b), nil
}

func (r *response) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

type messageStorage struct {
	messagesLock sync.Mutex
	messages     map[string]*buses.Message
}

func (ms *messageStorage) Put(m *buses.Message) {
	key := m.Headers[uniqueFanoutHeader]

	ms.messagesLock.Lock()
	defer ms.messagesLock.Unlock()

	ms.messages[key] = m
}

func (ms *messageStorage) pull(key string) (*buses.Message, error) {
	ms.messagesLock.Lock()
	defer ms.messagesLock.Unlock()

	if m, ok := ms.messages[key]; ok {
		delete(ms.messages, key)
		return m, nil
	} else {
		return nil, fmt.Errorf("could not find message: %v", key)
	}
}

func addTrackingHeader(r *http.Request) string {
	// Use two random 63 bit ints, as a poor approximation of a UUID.
	key := fmt.Sprintf("%X-%X", rand.Int63(), rand.Int63())
	r.Header.Set(uniqueFanoutHeader, key)
	return key
}

func removeTrackingHeader(m *buses.Message) {
	delete(m.Headers, uniqueFanoutHeader)
}
