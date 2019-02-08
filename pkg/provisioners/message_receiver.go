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

package provisioners

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
)

// MessageReceiver starts a server to receive new messages for the channel dispatcher. The new
// message is emitted via the receiver function.
const (
	MessageReceiverPort = 8080
)

// Message receiver receives messages.
type MessageReceiver struct {
	receiverFunc    func(ChannelReference, *Message) error
	forwardHeaders  sets.String
	forwardPrefixes []string

	logger *zap.SugaredLogger
}

// NewMessageReceiver creates a message receiver passing new messages to the
// receiverFunc.
func NewMessageReceiver(receiverFunc func(ChannelReference, *Message) error, logger *zap.SugaredLogger) *MessageReceiver {
	receiver := &MessageReceiver{
		receiverFunc:    receiverFunc,
		forwardHeaders:  sets.NewString(forwardHeaders...),
		forwardPrefixes: forwardPrefixes,

		logger: logger,
	}
	return receiver
}

// Start begings to receive messages for the receiver.
//
// Only HTTP POST requests to the root path (/) are accepted. If other paths or
// methods are needed, use the HandleRequest method directly with another HTTP
// server.
//
// This method will block until a message is received on the stop channel.
func (r *MessageReceiver) Start(stopCh <-chan struct{}) error {
	svr := r.start()
	defer r.stop(svr)

	<-stopCh
	return nil
}

func (r *MessageReceiver) start() *http.Server {
	r.logger.Info("Starting web server")
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", MessageReceiverPort),
		Handler: r.handler(),
	}
	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			r.logger.Errorf("HttpServer: ListenAndServe() error: %v", err)
		}
	}()
	return srv
}

func (r *MessageReceiver) stop(srv *http.Server) {
	r.logger.Info("Shutdown web server")
	if err := srv.Shutdown(nil); err != nil {
		r.logger.Fatal(err)
	}
}

// handler creates the http.Handler used by the http.Server started in MessageReceiver.Run.
func (r *MessageReceiver) handler() http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			res.WriteHeader(http.StatusNotFound)
			return
		}
		if req.Method != http.MethodPost {
			res.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		r.HandleRequest(res, req)
	})
}

// HandleRequest is an http.Handler function. The request is converted to a
// Message and emitted to the receiver func.
//
// The response status codes:
//   202 - the message was sent to subscribers
//   404 - the request was for an unknown channel
//   500 - an error occurred processing the request
func (r *MessageReceiver) HandleRequest(res http.ResponseWriter, req *http.Request) {
	host := req.Host
	r.logger.Infof("Received request for %s", host)
	channel, err := ParseChannel(host)
	if err != nil {
		r.logger.Info("Could not extract channel", zap.Error(err))
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	message, err := r.fromRequest(req)
	if err != nil {
		res.WriteHeader(http.StatusInternalServerError)
		return
	}
	// setting common channel information in the request
	message.AppendToHistory(host)

	err = r.receiverFunc(channel, message)
	if err != nil {
		if err == ErrUnknownChannel {
			res.WriteHeader(http.StatusNotFound)
		} else {
			res.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	res.WriteHeader(http.StatusAccepted)
}

func (r *MessageReceiver) fromRequest(req *http.Request) (*Message, error) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	headers := r.fromHTTPHeaders(req.Header)
	message := &Message{
		Headers: headers,
		Payload: body,
	}
	return message, nil
}

// fromHTTPHeaders converts HTTP headers into a message header map.
//
// Only headers whitelisted as safe are copied. If an HTTP header exists
// multiple times, a single value will be retained.
func (r *MessageReceiver) fromHTTPHeaders(headers http.Header) map[string]string {
	safe := map[string]string{}

	// TODO handle multi-value headers
	for h, v := range headers {
		// Headers are case-insensitive but test case are all lower-case
		comparable := strings.ToLower(h)
		if r.forwardHeaders.Has(comparable) {
			safe[h] = v[0]
			continue
		}
		for _, p := range r.forwardPrefixes {
			if strings.HasPrefix(comparable, p) {
				safe[h] = v[0]
				break
			}
		}
	}

	return safe
}

// ParseChannel converts the channel's hostname into a channel
// reference.
func ParseChannel(host string) (ChannelReference, error) {
	chunks := strings.Split(host, ".")
	if len(chunks) < 2 {
		return ChannelReference{}, fmt.Errorf("bad host format '%s'", host)
	}
	return ChannelReference{
		Name:      chunks[0],
		Namespace: chunks[1],
	}, nil
}
