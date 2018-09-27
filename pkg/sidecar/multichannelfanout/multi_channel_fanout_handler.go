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

package multichannelfanout

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing/pkg/buses"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"go.uber.org/zap"
	"net/http"
)

const (
	// The header injected into requests to state which `Channel` this request is being sent on. The
	// value is in the format created by multichannelfanout.MakeChannelKey().
	ChannelKeyHeader = "X-Channel-Key"
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

// MakeChannelKey creates the value to be sent in the HTTP header
// multichannelfanout.ChannelKeyHeader. This represents the `Channel` the request is being sent
// upon.
func MakeChannelKey(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

// makeChannelKeyFromConfig creates the channel key for a given channelConfig. It is a helper around
// MakeChannelKey.
func makeChannelKeyFromConfig(config ChannelConfig) string {
	return MakeChannelKey(config.Namespace, config.Name)
}

// getChannelKey extracts the channel key from the given HTTP request.
func getChannelKey(r *http.Request) string {
	cr := buses.ParseChannel(r.Host)
	return fmt.Sprintf("%s/%s", cr.Namespace, cr.Name)
}

// Handler is an http.Handler that looks in the HTTP headers of a request for the
// multichannelfanout.ChannelKeyHeader and uses its value to determine which fanout.Handler to
// delegate the request to.
type Handler struct {
	logger   *zap.Logger
	handlers map[string]http.Handler
	config   Config
}

func NewHandler(logger *zap.Logger, conf Config) (*Handler, error) {
	handlers := make(map[string]http.Handler, len(conf.ChannelConfigs))

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
	channelKey := getChannelKey(r)
	fh, ok := h.handlers[channelKey]
	if !ok {
		h.logger.Error("Unable to find a handler for request", zap.String("channelKey", channelKey))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fh.ServeHTTP(w, r)
}
