/*
Copyright 2019 The Knative Authors

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

package dispatcher

import (
	"fmt"
	"net/http"
	"sync"

	"k8s.io/apimachinery/pkg/labels"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	messaginglistersv1 "knative.dev/eventing/pkg/client/listers/messaging/v1"
)

const (
	readinessProbeReady    = http.StatusNoContent
	readinessProbeNotReady = http.StatusServiceUnavailable
	readinessProbeError    = http.StatusInternalServerError
)

// ReadinessChecker can assert the readiness of a component.
type ReadinessChecker interface {
	IsReady() (bool, error)
}

// DispatcherReadyChecker asserts the readiness of a dispatcher for in-memory Channels.
type DispatcherReadyChecker struct {
	// Allows listing observed Channels.
	chLister messaginglistersv1.InMemoryChannelLister

	// Allows listing/counting the handlers which have already been registered.
	chMsgHandler multichannelfanout.MultiChannelMessageHandler

	// Allows safe concurrent read/write of 'isReady'.
	sync.Mutex

	// Minor perf tweak, bypass check if we already observed readiness once.
	isReady bool
}

// IsReady implements ReadinessChecker.
// It checks whether the dispatcher has registered a handler for all observed in-memory Channels.
func (c *DispatcherReadyChecker) IsReady() (bool, error) {
	c.Lock()
	defer c.Unlock()

	// readiness already observed at an earlier point in time, short-circuit the check
	if c.isReady {
		return true, nil
	}

	channels, err := c.chLister.List(labels.Everything())
	if err != nil {
		return false, fmt.Errorf("listing cached InMemoryChannels: %w", err)
	}

	readyChannels := make([]*messagingv1.InMemoryChannel, 0, len(channels))
	for _, channel := range channels {
		if channel.IsReady() {
			readyChannels = append(readyChannels, channel)
		}
	}

	// Each channel creates two handler (http/https).
	c.isReady = c.chMsgHandler.CountChannelHandlers() >= 2*len(readyChannels)
	return c.isReady, nil
}

// readinessCheckerHTTPHandler returns a http.Handler which executes the given ReadinessChecker.
func readinessCheckerHTTPHandler(c ReadinessChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isReady, err := c.IsReady()
		switch {
		case err != nil:
			w.WriteHeader(readinessProbeError)
		case isReady:
			w.WriteHeader(readinessProbeReady)
		default:
			w.WriteHeader(readinessProbeNotReady)
		}
	}
}
