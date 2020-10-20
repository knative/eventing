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
// multichannelfanout.MessageHandler. When a new configuration is available, a new
// multichannelfanout.MessageHandler is created and swapped in. All subsequent requests go to the new
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

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
)

// Handler is an http.Handler that atomically swaps between underlying handlers.
type MessageHandler struct {
	// The current multichannelfanout.MessageHandler to delegate HTTP requests to. Never use this directly,
	// instead use {get,set}Handler, which enforces the type we expect.
	fanout     atomic.Value
	updateLock sync.Mutex
	logger     *zap.Logger
	reporter   channel.StatsReporter
}

// UpdateConfig updates the configuration to use the new config, returning an error if it can't.
type UpdateConfig func(config *multichannelfanout.Config) error

// NewMessageHandler creates a new swappable.Handler.
func NewMessageHandler(handler *multichannelfanout.MessageHandler, logger *zap.Logger, reporter channel.StatsReporter) *MessageHandler {
	h := &MessageHandler{
		logger:   logger.With(zap.String("httpHandler", "swappable")),
		reporter: reporter,
	}
	h.SetHandler(handler)
	return h
}

// NewEmptyMessageHandler creates a new swappable.Handler with an empty configuration.
func NewEmptyMessageHandler(context context.Context, logger *zap.Logger, messageDispatcher channel.MessageDispatcher, reporter channel.StatsReporter) (*MessageHandler, error) {
	h, err := multichannelfanout.NewMessageHandler(context, logger, messageDispatcher, multichannelfanout.Config{}, reporter)
	if err != nil {
		return nil, err
	}
	return NewMessageHandler(h, logger, reporter), nil
}

// GetHandler gets the current multichannelfanout.MessageHandler to delegate all HTTP
// requests to.
func (h *MessageHandler) GetHandler() *multichannelfanout.MessageHandler {
	return h.fanout.Load().(*multichannelfanout.MessageHandler)
}

// SetHandler sets a new multichannelfanout.MessageHandler to delegate all subsequent
// HTTP requests to.
func (h *MessageHandler) SetHandler(nh *multichannelfanout.MessageHandler) {
	h.fanout.Store(nh)
}

// UpdateConfig copies the current inner multichannelfanout.MessageHandler with the new configuration. If
// the new configuration is valid, then the new inner handler is swapped in and will start serving
// HTTP traffic.
func (h *MessageHandler) UpdateConfig(context context.Context, dispatcherConfig channel.EventDispatcherConfig, config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	h.updateLock.Lock()
	defer h.updateLock.Unlock()

	ih := h.GetHandler()
	if diff := ih.ConfigDiff(*config); diff != "" {
		h.logger.Info("Updating config (-old +new)", zap.String("diff", diff))
		newIh, err := ih.CopyWithNewConfig(context, dispatcherConfig, *config, h.reporter)
		if err != nil {
			h.logger.Info("Unable to update config", zap.Error(err), zap.Any("config", config))
			return err
		}
		h.SetHandler(newIh)
	}
	return nil
}

// ServeHTTP delegates all HTTP requests to the current multichannelfanout.MessageHandler.
func (h *MessageHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	// Hand work off to the current multi channel fanout handler.
	h.GetHandler().ServeHTTP(response, request)
}
