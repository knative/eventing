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

package inmemorychannel

import (
	"context"
	"time"

	"go.uber.org/zap"

	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"
)

type EventDispatcher interface {
	GetHandler(ctx context.Context) multichannelfanout.MultiChannelEventHandler
}

type InMemoryEventDispatcher struct {
	handler              multichannelfanout.MultiChannelEventHandler
	httpBindingsReceiver *kncloudevents.HTTPEventReceiver
	writeTimeout         time.Duration
	logger               *zap.Logger
}

type InMemoryEventDispatcherArgs struct {
	Port                     int
	ReadTimeout              time.Duration
	WriteTimeout             time.Duration
	Handler                  multichannelfanout.MultiChannelEventHandler
	Logger                   *zap.Logger
	HTTPEventReceiverOptions []kncloudevents.HTTPEventReceiverOption
}

// GetHandler gets the current multichannelfanout.EventHandler to delegate all HTTP
// requests to.
func (d *InMemoryEventDispatcher) GetHandler(ctx context.Context) multichannelfanout.MultiChannelEventHandler {
	return d.handler
}

func (d *InMemoryEventDispatcher) GetReceiver() kncloudevents.HTTPEventReceiver {
	return *d.httpBindingsReceiver
}

// Start starts the inmemory dispatcher's message processing.
// This is a blocking call.
func (d *InMemoryEventDispatcher) Start(ctx context.Context) error {
	return d.httpBindingsReceiver.StartListen(kncloudevents.WithShutdownTimeout(ctx, d.writeTimeout), d.handler)
}

// WaitReady blocks until the dispatcher's server is ready to receive requests.
func (d *InMemoryEventDispatcher) WaitReady() {
	<-d.httpBindingsReceiver.Ready
}

func NewEventDispatcher(args *InMemoryEventDispatcherArgs) *InMemoryEventDispatcher {
	// TODO set read timeouts?
	bindingsReceiver := kncloudevents.NewHTTPEventReceiver(
		args.Port,
		args.HTTPEventReceiverOptions...,
	)

	dispatcher := &InMemoryEventDispatcher{
		handler:              args.Handler,
		httpBindingsReceiver: bindingsReceiver,
		logger:               args.Logger,
		writeTimeout:         args.WriteTimeout,
	}

	return dispatcher
}
