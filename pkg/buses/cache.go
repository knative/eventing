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
		channelHosts:  make(map[ChannelHostReference]ChannelReference),
		subscriptions: make(map[SubscriptionReference]*channelsv1alpha1.Subscription),
	}
}

// Cache able to save and retrive Channels and Subscriptions by their
// reference. It is used by the reconciler to track which resources have been
// provisioned and comparing updated resources to the provisioned version.
type Cache struct {
	channels      map[ChannelReference]*channelsv1alpha1.Channel
	channelHosts  map[ChannelHostReference]ChannelReference
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

// ChannelHost returns a cached channel for a provided channel host reference
// or an error if the channel host is not in the cache.
func (c *Cache) ChannelHost(host ChannelHostReference) (*channelsv1alpha1.Channel, error) {
	ref, ok := c.channelHosts[host]
	if !ok {
		return nil, fmt.Errorf("unknown channel host %q", host.String())
	}
	return c.Channel(ref)
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
	if host, err := NewChannelHostReferenceFromChannel(channel); err == nil {
		// an error is expected if the channel is not serviceable yet
		c.channelHosts[host] = ref
	}
}

// RemoveChannel removes the provided channel from the cache.
func (c *Cache) RemoveChannel(channel *channelsv1alpha1.Channel) {
	if channel == nil {
		return
	}
	ref := NewChannelReference(channel)
	delete(c.channels, ref)
	if host, err := NewChannelHostReferenceFromChannel(channel); err != nil {
		// it's ok if key is abandoned in the channelHost cache after being
		// removed from the channel cache, but we should try to clean it up.
		delete(c.channelHosts, host)
	}
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
