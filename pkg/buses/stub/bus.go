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

func NewStubBusDispatcher(busRef buses.BusReference, opts *buses.BusOpts) *StubBus {
	bus := &StubBus{
		channels: make(map[buses.ChannelReference]*stubChannel),
	}
	handlerFuncs := buses.EventHandlerFuncs{
		ProvisionFunc: func(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Provision channel %q", channelRef.String())
			bus.addChannel(channelRef, parameters)
			return nil
		},
		UnprovisionFunc: func(channelRef buses.ChannelReference) error {
			bus.logger.Infof("Unprovision channel %q", channelRef.String())
			bus.removeChannel(channelRef)
			return nil
		},
		SubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			bus.logger.Infof("Subscribe %q to %q channel", subscriptionRef.String(), channelRef.String())
			bus.channel(channelRef).addSubscription(subscriptionRef, parameters, bus.dispatcher)
			return nil
		},
		UnsubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
			bus.logger.Infof("Unsubscribe %q from %q channel", subscriptionRef.String(), channelRef.String())
			bus.channel(channelRef).removeSubscription(subscriptionRef)
			return nil
		},
		ReceiveMessageFunc: func(channelRef buses.ChannelReference, message *buses.Message) error {
			bus.logger.Infof("Recieved message for %q channel", channelRef.String())
			bus.channel(channelRef).receiveMessage(message)
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(busRef, handlerFuncs, opts)
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

func (b *StubBus) addChannel(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) {
	if channel, ok := b.channels[channelRef]; ok {
		// update channel
		channel.parameters = parameters
	} else {
		// create channel
		b.channels[channelRef] = &stubChannel{
			parameters:    parameters,
			subscriptions: make(map[buses.SubscriptionReference]*stubSubscription),
			logger:        b.logger.With(zap.String("channels.knative.dev/channel", channelRef.String())),
		}
	}
}

func (b *StubBus) removeChannel(channelRef buses.ChannelReference) {
	delete(b.channels, channelRef)
}

func (b *StubBus) channel(channelRef buses.ChannelReference) *stubChannel {
	return b.channels[channelRef]
}

type stubChannel struct {
	channelRef    buses.ChannelReference
	parameters    buses.ResolvedParameters
	subscriptions map[buses.SubscriptionReference]*stubSubscription

	logger *zap.SugaredLogger
}

func (c *stubChannel) receiveMessage(message *buses.Message) {
	for _, stubSubscription := range c.subscriptions {
		go stubSubscription.dispatchMessage(message)
	}
}

func (c *stubChannel) addSubscription(subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters, bus buses.BusDispatcher) {
	if subscription, ok := c.subscriptions[subscriptionRef]; ok {
		// update subscription
		subscription.parameters = parameters
	} else {
		// create subscription
		c.subscriptions[subscriptionRef] = &stubSubscription{
			bus:             bus,
			parameters:      parameters,
			subscriptionRef: subscriptionRef,

			logger: c.logger.With(zap.String("channels.knative.dev/subscription", subscriptionRef.String())),
		}
	}
}

func (c *stubChannel) removeSubscription(subscriptionRef buses.SubscriptionReference) {
	delete(c.subscriptions, subscriptionRef)
}

type stubSubscription struct {
	bus             buses.BusDispatcher
	parameters      buses.ResolvedParameters
	subscriptionRef buses.SubscriptionReference

	logger *zap.SugaredLogger
}

func (s *stubSubscription) dispatchMessage(message *buses.Message) error {
	err := s.bus.DispatchMessage(s.subscriptionRef, message)
	if err != nil {
		s.logger.Warnf("Failed to dispatch message: %v", err)
	}
	return err
}
