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
// It is made up of a map, map[channel]fanout.Handler and each incoming request is inspected to
// determine which Channel it is on. This Handler delegates the HTTP handling to the fanout.Handler
// corresponding to the incoming request's Channel.
// It is often used in conjunction with a swappable.Handler. The swappable.Handler delegates all its
// requests to the multichannelfanout.Handler. When a new configuration is available, a new
// multichannelfanout.Handler is created and swapped in for all subsequent requests. The old
// multichannelfanout.Handler is discarded.
package multichannelfanout

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel/fanout"
)

// Handler is an http.Handler that introspects the incoming request to determine what Channel it is
// on, and then delegates handling of that request to the single fanout.Handler corresponding to
// that Channel.
type MessageHandler struct {
	logger   *zap.Logger
	handlers map[string]*fanout.MessageHandler
	config   Config
}

// NewHandler creates a new Handler.
func NewMessageHandler(ctx context.Context, logger *zap.Logger, conf Config) (*MessageHandler, error) {
	handlers := make(map[string]*fanout.MessageHandler, len(conf.ChannelConfigs))

	for _, cc := range conf.ChannelConfigs {
		key := makeChannelKeyFromConfig(cc)
		handler, err := fanout.NewMessageHandler(logger, cc.FanoutConfig)
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

	return &MessageHandler{
		logger:   logger,
		config:   conf,
		handlers: handlers,
	}, nil
}

// ConfigDiffs diffs the new config with the existing config. If there are no differences, then the
// empty string is returned. If there are differences, then a non-empty string is returned
// describing the differences.
func (h *MessageHandler) ConfigDiff(updated Config) string {
	return cmp.Diff(h.config, updated)
}

// CopyWithNewConfig creates a new copy of this Handler with all the fields identical, except the
// new Handler uses conf, rather than copying the existing Handler's config.
func (h *MessageHandler) CopyWithNewConfig(ctx context.Context, conf Config) (*MessageHandler, error) {
	return NewMessageHandler(ctx, h.logger, conf)
}

// ServeHTTP delegates the actual handling of the request to a fanout.Handler, based on the
// request's channel key.
func (h *MessageHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	channelKey := request.Host
	fh, ok := h.handlers[channelKey]
	if !ok {
		h.logger.Info("Unable to find a handler for request", zap.String("channelKey", channelKey))
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	fh.ServeHTTP(response, request)
}
