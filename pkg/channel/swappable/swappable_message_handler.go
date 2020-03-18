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

// Package swappable provides an http.Handler that delegates all HTTP requests to an underlying
// multichannelfanout.Handler. When a new configuration is available, a new
// multichannelfanout.Handler is created and swapped in. All subsequent requests go to the new
// handler.
// It is often used in conjunction with something that notices changes to ConfigMaps, such as
// configmap.watcher or configmap.filesystem.
package swappable

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel/multichannelfanout"
)

// Handler is an http.Handler that atomically swaps between underlying handlers.
type MessageHandler struct {
	// The current multichannelfanout.Handler to delegate HTTP requests to. Never use this directly,
	// instead use {get,set}MultiChannelFanoutHandler, which enforces the type we expect.
	fanout     atomic.Value
	updateLock sync.Mutex
	logger     *zap.Logger
}

// NewMessageHandler creates a new swappable.Handler.
func NewMessageHandler(handler *multichannelfanout.MessageHandler, logger *zap.Logger) *MessageHandler {
	h := &MessageHandler{
		logger: logger.With(zap.String("httpHandler", "swappable")),
	}
	h.setMultiChannelFanoutHandler(handler)
	return h
}

// NewEmptyMessageHandler creates a new swappable.Handler with an empty configuration.
func NewEmptyMessageHandler(context context.Context, logger *zap.Logger) (*MessageHandler, error) {
	h, err := multichannelfanout.NewMessageHandler(context, logger, multichannelfanout.Config{})
	if err != nil {
		return nil, err
	}
	return NewMessageHandler(h, logger), nil
}

// getMultiChannelFanoutHandler gets the current multichannelfanout.Handler to delegate all HTTP
// requests to.
func (h *MessageHandler) getMultiChannelFanoutHandler() *multichannelfanout.MessageHandler {
	return h.fanout.Load().(*multichannelfanout.MessageHandler)
}

// setMultiChannelFanoutHandler sets a new multichannelfanout.Handler to delegate all subsequent
// HTTP requests to.
func (h *MessageHandler) setMultiChannelFanoutHandler(nh *multichannelfanout.MessageHandler) {
	h.fanout.Store(nh)
}

// UpdateConfig copies the current inner multichannelfanout.Handler with the new configuration. If
// the new configuration is valid, then the new inner handler is swapped in and will start serving
// HTTP traffic.
func (h *MessageHandler) UpdateConfig(context context.Context, config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	h.updateLock.Lock()
	defer h.updateLock.Unlock()

	ih := h.getMultiChannelFanoutHandler()
	if diff := ih.ConfigDiff(*config); diff != "" {
		h.logger.Info("Updating config (-old +new)", zap.String("diff", diff))
		newIh, err := ih.CopyWithNewConfig(context, *config)
		if err != nil {
			h.logger.Info("Unable to update config", zap.Error(err), zap.Any("config", config))
			return err
		}
		h.setMultiChannelFanoutHandler(newIh)
	}
	return nil
}

// ServeHTTP delegates all HTTP requests to the current multichannelfanout.Handler.
func (h *MessageHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	// Hand work off to the current multi channel fanout handler.
	h.logger.Debug("ServeHTTP request received")
	h.getMultiChannelFanoutHandler().ServeHTTP(response, request)
}
