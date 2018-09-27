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

package swappable

import (
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"go.uber.org/zap"
	"net/http"
	"sync/atomic"
)

// http.Handler that atomically swapping between underlying handlers.
type Handler struct {
	// The current multichannelfanout.Handler to delegate HTTP requests to. Never use this directly,
	// instead use {get,set}MultiChannelFanoutHandler, which enforces the type we expect.
	fanout atomic.Value
	logger *zap.Logger
}

// NewHandler creates a new swappable.Handler.
func NewHandler(handler *multichannelfanout.Handler, logger *zap.Logger) (*Handler, error) {
	h := &Handler{
		logger: logger.With(zap.String("httpHandler", "swappable")),
	}
	h.SetMultiChannelFanoutHandler(handler)
	return h, nil
}

func NewEmptyHandler(logger *zap.Logger) (*Handler, error) {
	h, err := multichannelfanout.NewHandler(logger, multichannelfanout.Config{})
	if err != nil {
		return nil, err
	}
	return NewHandler(h, logger)
}

// getMultiChannelFanoutHandler gets the current multichannelfanout.Handler to delegate all HTTP
// requests to.
func (h *Handler) GetMultiChannelFanoutHandler() *multichannelfanout.Handler {
	return h.fanout.Load().(*multichannelfanout.Handler)
}

// setMultiChannelFanoutHandler sets a new multichannelfanout.Handler to delegate all subsequent
// HTTP requests to.
func (h *Handler) SetMultiChannelFanoutHandler(new *multichannelfanout.Handler) {
	h.fanout.Store(new)
}

// ServeHTTP delegates all HTTP requests to the current multichannelfanout.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Hand work off to the current multi channel fanout handler.
	h.logger.Debug("ServeHTTP request received")
	h.GetMultiChannelFanoutHandler().ServeHTTP(w, r)
}
