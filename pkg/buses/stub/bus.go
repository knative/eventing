/*
 * Copyright 2018 The Knative Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package stub

import (
	"go.uber.org/zap"

	"github.com/knative/eventing/pkg/buses"
)

// BusType is the type of the stub bus
const BusType = "stub"

func NewStubBusDispatcher(ref buses.BusReference, opts *buses.BusOpts) *StubBus {
	bus := &StubBus{
		channels: make(map[buses.ChannelReference]*stubChannel),
	}
	handlerFuncs := buses.EventHandlerFuncs{
		ProvisionFunc: func(channel buses.ChannelReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Provision channel %q", channel.String())
			bus.addChannel(channel, parameters)
			return nil
		},
		UnprovisionFunc: func(channel buses.ChannelReference) error {
			bus.logger.Infof("Unprovision channel %q", channel.String())
			bus.removeChannel(channel)
			return nil
		},
		SubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Subscribe %q to %q channel", subscription.String(), channel.String())
			bus.channel(channel).addSubscription(subscription, parameters, bus.dispatcher)
			return nil
		},
		UnsubscribeFunc: func(channel buses.ChannelReference, subscription buses.SubscriptionReference) error {
			bus.logger.Infof("Unsubscribe %q from %q channel", subscription.String(), channel.String())
			bus.channel(channel).removeSubscription(subscription)
			return nil
		},
		ReceiveMessageFunc: func(channel buses.ChannelReference, message *buses.Message) error {
			bus.logger.Infof("Recieved message for %q channel", channel.String())
			bus.channel(channel).receiveMessage(message)
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(ref, handlerFuncs, opts)
	bus.logger = opts.Logger

	return bus
}

type StubBus struct {
	dispatcher buses.BusDispatcher
	channels   map[buses.ChannelReference]*stubChannel

	logger *zap.SugaredLogger
}

func (b *StubBus) Run(threadness int, stopCh <-chan struct{}) {
	b.dispatcher.Run(threadness, stopCh)
}

func (b *StubBus) addChannel(ref buses.ChannelReference, parameters buses.ResolvedParameters) {
	if channel, ok := b.channels[ref]; ok {
		// update channel
		channel.parameters = parameters
	} else {
		// create channel
		b.channels[ref] = &stubChannel{
			ref:           ref,
			parameters:    parameters,
			subscriptions: make(map[buses.SubscriptionReference]*stubSubscription),
			logger:        b.logger.With(zap.String("channels.knative.dev/channel", ref.String())),
		}
	}
}

func (b *StubBus) removeChannel(channel buses.ChannelReference) {
	delete(b.channels, channel)
}

func (b *StubBus) channel(channel buses.ChannelReference) *stubChannel {
	return b.channels[channel]
}

type stubChannel struct {
	ref           buses.ChannelReference
	parameters    buses.ResolvedParameters
	subscriptions map[buses.SubscriptionReference]*stubSubscription

	logger *zap.SugaredLogger
}

func (c *stubChannel) receiveMessage(message *buses.Message) {
	for _, stubSubscription := range c.subscriptions {
		go stubSubscription.dispatchMessage(message)
	}
}

func (c *stubChannel) addSubscription(ref buses.SubscriptionReference, parameters buses.ResolvedParameters, bus buses.BusDispatcher) {
	if subscription, ok := c.subscriptions[ref]; ok {
		// update subscription
		subscription.parameters = parameters
	} else {
		// create subscription
		c.subscriptions[ref] = &stubSubscription{
			ref:        ref,
			bus:        bus,
			parameters: parameters,

			logger: c.logger.With(zap.String("channels.knative.dev/subscription", ref.String())),
		}
	}
}

func (c *stubChannel) removeSubscription(subscription buses.SubscriptionReference) {
	delete(c.subscriptions, subscription)
}

type stubSubscription struct {
	ref        buses.SubscriptionReference
	bus        buses.BusDispatcher
	parameters buses.ResolvedParameters

	logger *zap.SugaredLogger
}

func (s *stubSubscription) dispatchMessage(message *buses.Message) error {
	err := s.bus.DispatchMessage(s.ref, message)
	if err != nil {
		s.logger.Warnf("Failed to dispatch message: %v", err)
	}
	return err
}
