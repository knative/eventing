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

	"knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/channel/swappable"
	"knative.dev/eventing/pkg/kncloudevents"
)

type MessageDispatcher interface {
	UpdateConfig(ctx context.Context, dispatcherConfig channel.EventDispatcherConfig, config *multichannelfanout.Config) error
}

type InMemoryMessageDispatcher struct {
	handler              *swappable.MessageHandler
	httpBindingsReceiver *kncloudevents.HttpMessageReceiver
	writeTimeout         time.Duration
	logger               *zap.Logger
}

type InMemoryMessageDispatcherArgs struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Handler      *swappable.MessageHandler
	Logger       *zap.Logger
}

func (d *InMemoryMessageDispatcher) UpdateConfig(ctx context.Context, dispatcherConfig channel.EventDispatcherConfig, config *multichannelfanout.Config) error {
	return d.handler.UpdateConfig(ctx, dispatcherConfig, config)
}

// Start starts the inmemory dispatcher's message processing.
// This is a blocking call.
func (d *InMemoryMessageDispatcher) Start(ctx context.Context) error {
	return d.httpBindingsReceiver.StartListen(kncloudevents.WithShutdownTimeout(ctx, d.writeTimeout), d.handler)
}

func NewMessageDispatcher(args *InMemoryMessageDispatcherArgs) *InMemoryMessageDispatcher {
	// TODO set read timeouts?
	bindingsReceiver := kncloudevents.NewHttpMessageReceiver(args.Port)

	dispatcher := &InMemoryMessageDispatcher{
		handler:              args.Handler,
		httpBindingsReceiver: bindingsReceiver,
		logger:               args.Logger,
		writeTimeout:         args.WriteTimeout,
	}

	return dispatcher
}
