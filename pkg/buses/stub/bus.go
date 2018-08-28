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
	"github.com/golang/glog"

	"github.com/knative/eventing/pkg/buses"
)

func NewStubBusDispatcher(busRef buses.BusReference, opts *buses.BusOpts) *StubBus {
	bus := &StubBus{
		channels: make(map[buses.ChannelReference]*stubChannel),
	}
	handlerFuncs := buses.EventHandlerFuncs{
		ProvisionFunc: func(channelRef buses.ChannelReference, parameters buses.ResolvedParameters) error {
			glog.Infof("Provision channel %q\n", channelRef.String())
			bus.addChannel(channelRef, parameters)
			return nil
		},
		UnprovisionFunc: func(channelRef buses.ChannelReference) error {
			glog.Infof("Unprovision channel %q\n", channelRef.String())
			bus.removeChannel(channelRef)
			return nil
		},
		SubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters) error {
			glog.Infof("Subscribe %q to %q channel\n", subscriptionRef.String(), channelRef.String())
			bus.channel(channelRef).addSubscription(subscriptionRef, parameters, bus.dispatcher)
			return nil
		},
		UnsubscribeFunc: func(channelRef buses.ChannelReference, subscriptionRef buses.SubscriptionReference) error {
			glog.Infof("Unsubscribe %q from %q channel\n", subscriptionRef.String(), channelRef.String())
			bus.channel(channelRef).removeSubscription(subscriptionRef)
			return nil
		},
		ReceiveMessageFunc: func(channelRef buses.ChannelReference, message *buses.Message) error {
			glog.Infof("Recieved message for %q channel\n", channelRef.String())
			bus.channel(channelRef).receiveMessage(message)
			return nil
		},
	}
	bus.dispatcher = buses.NewBusDispatcher(busRef, handlerFuncs, opts)

	return bus
}

type StubBus struct {
	dispatcher buses.BusDispatcher
	channels   map[buses.ChannelReference]*stubChannel
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
}

func (c *stubChannel) receiveMessage(message *buses.Message) {
	for _, stubSubscription := range c.subscriptions {
		go stubSubscription.dispatchMessage(message)
	}
}

func (c *stubChannel) addSubscription(subscriptionRef buses.SubscriptionReference, parameters buses.ResolvedParameters, bus buses.BusDispatcher) {
	// create or update subscription
	c.subscriptions[subscriptionRef] = &stubSubscription{
		bus:             bus,
		parameters:      parameters,
		subscriptionRef: subscriptionRef,
	}
}

func (c *stubChannel) removeSubscription(subscriptionRef buses.SubscriptionReference) {
	delete(c.subscriptions, subscriptionRef)
}

type stubSubscription struct {
	bus             buses.BusDispatcher
	parameters      buses.ResolvedParameters
	subscriptionRef buses.SubscriptionReference
}

func (s *stubSubscription) dispatchMessage(message *buses.Message) error {
	return s.bus.DispatchMessage(s.subscriptionRef, message)
}
