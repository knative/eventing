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

// Package swappable provides an http.Handler that delegates all HTTP requests to an underlying
// multichannelfanout.Handler. When a new configuration is available, a new
// multichannelfanout.Handler is created and swapped in. All subsequent requests go to the new
// handler.
// It is often used in conjunction with something that notices changes to ConfigMaps, such as
// configmap.watcher or configmap.filesystem.
package swappable

import (
	"errors"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
	"net/http"
	"sync"
	"sync/atomic"
)

// http.Handler that atomically swaps between underlying handlers.
type Handler struct {
	// The current multichannelfanout.Handler to delegate HTTP requests to. Never use this directly,
	// instead use {get,set}MultiChannelFanoutHandler, which enforces the type we expect.
	fanout     atomic.Value
	updateLock sync.Mutex
	logger     *zap.Logger
}

type UpdateConfig func(config *multichannelfanout.Config) error

var _ UpdateConfig = (&Handler{}).UpdateConfig

// NewHandler creates a new swappable.Handler.
func NewHandler(handler *multichannelfanout.Handler, logger *zap.Logger) *Handler {
	h := &Handler{
		logger: logger.With(zap.String("httpHandler", "swappable")),
	}
	h.setMultiChannelFanoutHandler(handler)
	return h
}

func NewEmptyHandler(logger *zap.Logger) (*Handler, error) {
	h, err := multichannelfanout.NewHandler(logger, multichannelfanout.Config{})
	if err != nil {
		return nil, err
	}
	return NewHandler(h, logger), nil
}

// getMultiChannelFanoutHandler gets the current multichannelfanout.Handler to delegate all HTTP
// requests to.
func (h *Handler) getMultiChannelFanoutHandler() *multichannelfanout.Handler {
	return h.fanout.Load().(*multichannelfanout.Handler)
}

// setMultiChannelFanoutHandler sets a new multichannelfanout.Handler to delegate all subsequent
// HTTP requests to.
func (h *Handler) setMultiChannelFanoutHandler(nh *multichannelfanout.Handler) {
	h.fanout.Store(nh)
}

// UpdateConfig copies the current inner multichannelfanout.Handler with the new configuration. If
// the new configuration is valid, then the new inner handler is swapped in and will start serving
// HTTP traffic.
func (h *Handler) UpdateConfig(config *multichannelfanout.Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	h.updateLock.Lock()
	defer h.updateLock.Unlock()

	ih := h.getMultiChannelFanoutHandler()
	if diff := ih.ConfigDiff(*config); diff != "" {
		h.logger.Info("Updating config (-old +new)", zap.String("diff", diff))
		newIh, err := ih.CopyWithNewConfig(*config)
		if err != nil {
			h.logger.Info("Unable to update config", zap.Error(err), zap.Any("config", config))
			return err
		}
		h.setMultiChannelFanoutHandler(newIh)
	}
	return nil
}

// ServeHTTP delegates all HTTP requests to the current multichannelfanout.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Hand work off to the current multi channel fanout handler.
	h.logger.Debug("ServeHTTP request received")
	h.getMultiChannelFanoutHandler().ServeHTTP(w, r)
}
