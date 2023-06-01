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

// Package multichannelfanout provides an http.Handler that takes in one request to a Knative
// Channel and fans it out to N other requests. Logically, it represents multiple Knative Channels.
// It is made up of a map, map[channel]fanout.MessageHandler and each incoming request is inspected to
// determine which Channel it is on. This Handler delegates the HTTP handling to the fanout.MessageHandler
// corresponding to the incoming request's Channel.
// It is often used in conjunction with a swappable.Handler. The swappable.Handler delegates all its
// requests to the multichannelfanout.MessageHandler. When a new configuration is available, a new
// multichannelfanout.MessageHandler is created and swapped in for all subsequent requests. The old
// multichannelfanout.MessageHandler is discarded.
package multichannelfanout

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
)

type MultiChannelMessageHandler interface {
	http.Handler
	SetChannelHandler(host string, handler fanout.MessageHandler)
	DeleteChannelHandler(host string)
	GetChannelHandler(host string) fanout.MessageHandler
	CountChannelHandlers() int
}

// Handler is an http.Handler that introspects the incoming request to determine what Channel it is
// on, and then delegates handling of that request to the single fanout.FanoutMessageHandler corresponding to
// that Channel.
type MessageHandler struct {
	logger       *zap.Logger
	handlersLock sync.RWMutex
	handlers     map[string]fanout.MessageHandler
}

// NewHandler creates a new Handler.
func NewMessageHandler(_ context.Context, logger *zap.Logger) *MessageHandler {
	return &MessageHandler{
		logger:   logger,
		handlers: make(map[string]fanout.MessageHandler),
	}
}

// NewMessageHandlerWithConfig creates a new Handler with the specified configuration. This is really meant for tests
// where you want to apply a fully specified configuration for tests. Reconciler operates on single channel at a time.
func NewMessageHandlerWithConfig(_ context.Context, logger *zap.Logger, messageDispatcher channel.MessageDispatcher, conf Config, reporter channel.StatsReporter, recvOptions ...channel.MessageReceiverOptions) (*MessageHandler, error) {
	handlers := make(map[string]fanout.MessageHandler, len(conf.ChannelConfigs))

	for _, cc := range conf.ChannelConfigs {
		keys := []string{cc.HostName, cc.Path}
		for _, key := range keys {
			if key == "" {
				continue
			}
			handler, err := fanout.NewFanoutMessageHandler(logger, messageDispatcher, cc.FanoutConfig, reporter, recvOptions...)
			if err != nil {
				logger.Error("Failed creating new fanout handler.", zap.Error(err))
				return nil, err
			}
			if _, present := handlers[key]; present {
				logger.Error("Duplicate channel key", zap.String("channelKey", key))
				return nil, fmt.Errorf("duplicate channel key: %v", key)
			}
			handlers[key] = handler
		}
	}
	return &MessageHandler{
		logger:   logger,
		handlers: handlers,
	}, nil
}

func (h *MessageHandler) SetChannelHandler(host string, handler fanout.MessageHandler) {
	h.handlersLock.Lock()
	defer h.handlersLock.Unlock()
	h.handlers[host] = handler
}

func (h *MessageHandler) DeleteChannelHandler(host string) {
	h.handlersLock.Lock()
	defer h.handlersLock.Unlock()
	delete(h.handlers, host)
}

func (h *MessageHandler) GetChannelHandler(host string) fanout.MessageHandler {
	h.handlersLock.RLock()
	defer h.handlersLock.RUnlock()
	return h.handlers[host]
}

func (h *MessageHandler) CountChannelHandlers() int {
	h.handlersLock.RLock()
	defer h.handlersLock.RUnlock()
	return len(h.handlers)
}

// ServeHTTP delegates the actual handling of the request to a fanout.MessageHandler, based on the
// request's channel key.
func (h *MessageHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	channelKey := request.Host

	if request.URL.Path != "/" {
		channelRef, err := channel.ParseChannelFromPath(request.URL.Path)
		if err != nil {
			h.logger.Error("unable to retrieve channel from path")
			response.WriteHeader(http.StatusBadRequest)
			return
		}
		channelKey = fmt.Sprintf("%s/%s", channelRef.Namespace, channelRef.Name)
	}

	fh := h.GetChannelHandler(channelKey)
	if fh == nil {
		h.logger.Info("Unable to find a handler for request", zap.String("channelKey", channelKey))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	fh.ServeHTTP(response, request)
}
