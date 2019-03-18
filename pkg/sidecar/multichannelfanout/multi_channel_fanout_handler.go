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
	"fmt"
	"net/http"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"go.uber.org/zap"
)

// The configuration of this handler.
type Config struct {
	// The configuration of each channel in this handler.
	ChannelConfigs []ChannelConfig `json:"channelConfigs"`
}

type ChannelConfig struct {
	Namespace    string        `json:"namespace"`
	Name         string        `json:"name"`
	FanoutConfig fanout.Config `json:"fanoutConfig"`
}

// MakeChannelKey creates the key used for this Channel in the Handler's handlers map.
func makeChannelKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// makeChannelKeyFromConfig creates the channel key for a given channelConfig. It is a helper around
// MakeChannelKey.
func makeChannelKeyFromConfig(config ChannelConfig) string {
	return makeChannelKey(config.Namespace, config.Name)
}

// getChannelKey extracts the channel key from the given HTTP request.
func getChannelKey(r *http.Request) (string, error) {
	cr, err := provisioners.ParseChannel(r.Host)
	if err != nil {
		return "", err
	}
	return makeChannelKey(cr.Namespace, cr.Name), nil
}

// Handler is an http.Handler that introspects the incoming request to determine what Channel it is
// on, and then delegates handling of that request to the single fanout.Handler corresponding to
// that Channel.
type Handler struct {
	logger   *zap.Logger
	handlers map[string]*fanout.Handler
	config   Config
}

// NewHandler creates a new Handler.
func NewHandler(logger *zap.Logger, conf Config) (*Handler, error) {
	handlers := make(map[string]*fanout.Handler, len(conf.ChannelConfigs))

	for _, cc := range conf.ChannelConfigs {
		key := makeChannelKeyFromConfig(cc)
		handler := fanout.NewHandler(logger, cc.FanoutConfig)
		if _, present := handlers[key]; present {
			logger.Error("Duplicate channel key", zap.String("channelKey", key))
			return nil, fmt.Errorf("duplicate channel key: %v", key)
		}
		handlers[key] = handler
	}

	return &Handler{
		logger:   logger,
		config:   conf,
		handlers: handlers,
	}, nil
}

// ConfigDiffs diffs the new config with the existing config. If there are no differences, then the
// empty string is returned. If there are differences, then a non-empty string is returned
// describing the differences.
func (h *Handler) ConfigDiff(updated Config) string {
	return cmp.Diff(h.config, updated)
}

// CopyWithNewConfig creates a new copy of this Handler with all the fields identical, except the
// new Handler uses conf, rather than copying the existing Handler's config.
func (h *Handler) CopyWithNewConfig(conf Config) (*Handler, error) {
	return NewHandler(h.logger, conf)
}

// ServeHTTP delegates the actual handling of the request to a fanout.Handler, based on the
// request's channel key.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	channelKey, err := getChannelKey(r)
	if err != nil {
		h.logger.Error("Unable to extract channelKey", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fh, ok := h.handlers[channelKey]
	if !ok {
		h.logger.Error("Unable to find a handler for request", zap.String("channelKey", channelKey))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fh.ServeHTTP(w, r)
}
