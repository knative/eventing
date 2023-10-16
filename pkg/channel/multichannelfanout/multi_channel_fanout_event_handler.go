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
// It is made up of a map, map[channel]fanout.EventHandler and each incoming request is inspected to
// determine which Channel it is on. This Handler delegates the HTTP handling to the fanout.EventHandler
// corresponding to the incoming request's Channel.
// It is often used in conjunction with a swappable.Handler. The swappable.Handler delegates all its
// requests to the multichannelfanout.EventHandler. When a new configuration is available, a new
// multichannelfanout.EventHandler is created and swapped in for all subsequent requests. The old
// multichannelfanout.EventHandler is discarded.
package multichannelfanout

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/kncloudevents"
)

type MultiChannelEventHandler interface {
	http.Handler
	SetChannelHandler(host string, handler fanout.EventHandler)
	DeleteChannelHandler(host string)
	GetChannelHandler(host string) fanout.EventHandler
	CountChannelHandlers() int
}

// Handler is an http.Handler that introspects the incoming request to determine what Channel it is
// on, and then delegates handling of that request to the single fanout.FanoutEventHandler corresponding to
// that Channel.
type EventHandler struct {
	logger       *zap.Logger
	handlersLock sync.RWMutex
	handlers     map[string]fanout.EventHandler
}

// NewHandler creates a new Handler.
func NewEventHandler(_ context.Context, logger *zap.Logger) *EventHandler {
	return &EventHandler{
		logger:   logger,
		handlers: make(map[string]fanout.EventHandler),
	}
}

// NewEventHandlerWithConfig creates a new Handler with the specified configuration. This is really meant for tests
// where you want to apply a fully specified configuration for tests. Reconciler operates on single channel at a time.
func NewEventHandlerWithConfig(_ context.Context, logger *zap.Logger, conf Config, reporter channel.StatsReporter, eventDispatcher *kncloudevents.Dispatcher, recvOptions ...channel.EventReceiverOptions) (*EventHandler, error) {
	handlers := make(map[string]fanout.EventHandler, len(conf.ChannelConfigs))

	for _, cc := range conf.ChannelConfigs {
		keys := []string{cc.HostName, cc.Path}
		for _, key := range keys {
			if key == "" {
				continue
			}
			handler, err := fanout.NewFanoutEventHandler(logger, cc.FanoutConfig, reporter, cc.EventTypeHandler, cc.ChannelAddressable, cc.ChannelUID, eventDispatcher, recvOptions...)
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
	return &EventHandler{
		logger:   logger,
		handlers: handlers,
	}, nil
}

func (h *EventHandler) SetChannelHandler(host string, handler fanout.EventHandler) {
	h.handlersLock.Lock()
	defer h.handlersLock.Unlock()
	h.handlers[host] = handler
}

func (h *EventHandler) DeleteChannelHandler(host string) {
	h.handlersLock.Lock()
	defer h.handlersLock.Unlock()
	delete(h.handlers, host)
}

func (h *EventHandler) GetChannelHandler(host string) fanout.EventHandler {
	h.handlersLock.RLock()
	defer h.handlersLock.RUnlock()
	return h.handlers[host]
}

func (h *EventHandler) CountChannelHandlers() int {
	h.handlersLock.RLock()
	defer h.handlersLock.RUnlock()
	return len(h.handlers)
}

// ServeHTTP delegates the actual handling of the request to a fanout.EventHandler, based on the
// request's channel key.
func (h *EventHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
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
