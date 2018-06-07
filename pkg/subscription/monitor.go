/*
 * Copyright 2018 the original author or authors.
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

package subscription

import (
	"reflect"
	"sync"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

type Monitor struct {
	busName string
	handler MonitorEventHandlerFuncs

	cache map[channelKey]*channelSummary
	mutex *sync.Mutex
}

type MonitorEventHandlerFuncs struct {
	ProvisionFunc   func(channel channelsv1alpha1.Channel)
	UnprovisionFunc func(channel channelsv1alpha1.Channel)
	SubscribeFunc   func(subscription channelsv1alpha1.Subscription)
	UnsubscribeFunc func(subscription channelsv1alpha1.Subscription)
}

type channelSummary struct {
	Channel       *channelsv1alpha1.ChannelSpec
	Subscriptions map[subscriptionKey]subscriptionSummary
}

type subscriptionSummary struct {
	Subscription channelsv1alpha1.SubscriptionSpec
}

func NewMonitor(busName string, informerFactory informers.SharedInformerFactory, handler MonitorEventHandlerFuncs) *Monitor {

	channelInformer := informerFactory.Channels().V1alpha1().Channels()
	subscriptionInformer := informerFactory.Channels().V1alpha1().Subscriptions()

	monitor := &Monitor{
		busName: busName,
		handler: handler,

		cache: make(map[channelKey]*channelSummary),
		mutex: &sync.Mutex{},
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Channel resources change
	channelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.createOrUpdateChannel(*channel)
		},
		UpdateFunc: func(old, new interface{}) {
			oldChannel := old.(*channelsv1alpha1.Channel)
			newChannel := new.(*channelsv1alpha1.Channel)

			if oldChannel.ResourceVersion == newChannel.ResourceVersion {
				// Periodic resync will send update events for all known Channels.
				// Two different versions of the same Channel will always have different RVs.
				return
			}

			monitor.createOrUpdateChannel(*newChannel)
			if oldChannel.Spec.Bus != newChannel.Spec.Bus {
				monitor.removeChannel(*oldChannel)
			}
		},
		DeleteFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.removeChannel(*channel)
		},
	})
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.createOrUpdateSubscription(*subscription)
		},
		UpdateFunc: func(old, new interface{}) {
			oldSubscription := old.(*channelsv1alpha1.Subscription)
			newSubscription := new.(*channelsv1alpha1.Subscription)

			if oldSubscription.ResourceVersion == newSubscription.ResourceVersion {
				// Periodic resync will send update events for all known Subscriptions.
				// Two different versions of the same Subscription will always have different RVs.
				return
			}

			monitor.createOrUpdateSubscription(*newSubscription)
			if oldSubscription.Spec.Channel != newSubscription.Spec.Channel {
				monitor.removeSubscription(*oldSubscription)
			}
		},
		DeleteFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.removeSubscription(*subscription)
		},
	})

	return monitor
}

// Subscriptions for a channel name and namespace
func (m *Monitor) Subscriptions(channel string, namespace string) *[]channelsv1alpha1.SubscriptionSpec {
	channelKey := makeChannelKeyWithNames(channel, namespace)
	summary := m.getOrCreateChannelSummary(channelKey)

	if summary.Channel == nil {
		// the channel is unknown
		return nil
	}

	if summary.Channel.Bus != m.busName {
		// the channel is not for this bus
		return nil
	}

	m.mutex.Lock()
	subscriptions := make([]channelsv1alpha1.SubscriptionSpec, len(summary.Subscriptions))
	for _, subscription := range summary.Subscriptions {
		subscriptions = append(subscriptions, subscription.Subscription)
	}
	m.mutex.Unlock()

	return &subscriptions
}

func (m *Monitor) getOrCreateChannelSummary(key channelKey) *channelSummary {
	m.mutex.Lock()
	summary, ok := m.cache[key]
	if !ok {
		summary = &channelSummary{
			Channel:       nil,
			Subscriptions: make(map[subscriptionKey]subscriptionSummary),
		}
		m.cache[key] = summary
	}
	m.mutex.Unlock()

	return summary
}

func (m *Monitor) createOrUpdateChannel(channel channelsv1alpha1.Channel) {
	channelKey := makeChannelKeyFromChannel(channel)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	old := summary.Channel
	new := &channel.Spec
	summary.Channel = new
	m.mutex.Unlock()

	if !reflect.DeepEqual(old, new) {
		m.handler.ProvisionFunc(channel)
	}
}

func (m *Monitor) removeChannel(channel channelsv1alpha1.Channel) {
	channelKey := makeChannelKeyFromChannel(channel)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	summary.Channel = nil
	m.mutex.Unlock()

	m.handler.UnprovisionFunc(channel)
}

func (m *Monitor) createOrUpdateSubscription(subscription channelsv1alpha1.Subscription) {
	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getOrCreateChannelSummary(channelKey)
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)

	m.mutex.Lock()
	old := summary.Subscriptions[subscriptionKey]
	new := subscriptionSummary{
		Subscription: subscription.Spec,
	}
	summary.Subscriptions[subscriptionKey] = new
	m.mutex.Unlock()

	if !reflect.DeepEqual(old.Subscription, new.Subscription) {
		m.handler.SubscribeFunc(subscription)
	}
}

func (m *Monitor) removeSubscription(subscription channelsv1alpha1.Subscription) {
	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getOrCreateChannelSummary(channelKey)
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)

	m.mutex.Lock()
	delete(summary.Subscriptions, subscriptionKey)
	m.mutex.Unlock()

	m.handler.UnsubscribeFunc(subscription)
}

type channelKey struct {
	Name      string
	Namespace string
}

func makeChannelKeyFromChannel(channel channelsv1alpha1.Channel) channelKey {
	return makeChannelKeyWithNames(channel.Name, channel.Namespace)
}

func makeChannelKeyFromSubscription(subscription channelsv1alpha1.Subscription) channelKey {
	return makeChannelKeyWithNames(subscription.Spec.Channel, subscription.Namespace)
}

func makeChannelKeyWithNames(name string, namespace string) channelKey {
	return channelKey{
		Name:      name,
		Namespace: namespace,
	}
}

type subscriptionKey struct {
	Name      string
	Namespace string
}

func makeSubscriptionKeyFromSubscription(subscription channelsv1alpha1.Subscription) subscriptionKey {
	return subscriptionKey{
		Name:      subscription.Name,
		Namespace: subscription.Namespace,
	}
}
