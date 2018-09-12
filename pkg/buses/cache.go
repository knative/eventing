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

package buses

import (
	"fmt"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
)

// NewCache create a cache that is able to save and retrive Channels and
// Subscriptions by their reference.
func NewCache() *Cache {
	return &Cache{
		channels:      make(map[ChannelReference]*channelsv1alpha1.Channel),
		subscriptions: make(map[SubscriptionReference]*channelsv1alpha1.Subscription),
	}
}

// Cache able to save and retrive Channels and Subscriptions by their
// reference. It is used by the reconciler to track which resources have been
// provisioned and comparing updated resources to the provisioned version.
type Cache struct {
	channels      map[ChannelReference]*channelsv1alpha1.Channel
	subscriptions map[SubscriptionReference]*channelsv1alpha1.Subscription
}

// Channel returns a cached channel for provided reference or an error if the
// channel is not in the cache.
func (c *Cache) Channel(ref ChannelReference) (*channelsv1alpha1.Channel, error) {
	channel, ok := c.channels[ref]
	if !ok {
		return nil, fmt.Errorf("unknown channel %q", ref.String())
	}
	return channel, nil
}

// Subscription returns a cached subscription for provided reference or an
// error if the subscription is not in the cache.
func (c *Cache) Subscription(ref SubscriptionReference) (*channelsv1alpha1.Subscription, error) {
	subscription, ok := c.subscriptions[ref]
	if !ok {
		return nil, fmt.Errorf("unknown subscription %q", ref.String())
	}
	return subscription, nil
}

// AddChannel adds, or updates, the provided channel to the cache for later
// retrieal by its reference.
func (c *Cache) AddChannel(channel *channelsv1alpha1.Channel) {
	if channel == nil {
		return
	}
	ref := NewChannelReference(channel)
	c.channels[ref] = channel
}

// RemoveChannel removes the provided channel from the cache.
func (c *Cache) RemoveChannel(channel *channelsv1alpha1.Channel) {
	if channel == nil {
		return
	}
	ref := NewChannelReference(channel)
	delete(c.channels, ref)
}

// AddSubscription adds, or updates, the provided subscription to the cache for
// later retrieal by its reference.
func (c *Cache) AddSubscription(subscription *channelsv1alpha1.Subscription) {
	if subscription == nil {
		return
	}
	ref := NewSubscriptionReference(subscription)
	c.subscriptions[ref] = subscription
}

// RemoveSubscription removes the provided subscription from the cache.
func (c *Cache) RemoveSubscription(subscription *channelsv1alpha1.Subscription) {
	if subscription == nil {
		return
	}
	ref := NewSubscriptionReference(subscription)
	delete(c.subscriptions, ref)
}

func (c *Cache) AllChannels() []*channelsv1alpha1.Channel {
	chans := []*channelsv1alpha1.Channel{}
	for _, channel := range c.channels {
		chans = append(chans, channel)
	}
	return chans
}

func (c *Cache) AllSubscriptions() []*channelsv1alpha1.Subscription {
	subs := []*channelsv1alpha1.Subscription{}
	for _, sub := range c.subscriptions {
		subs = append(subs, sub)
	}
	return subs
}
