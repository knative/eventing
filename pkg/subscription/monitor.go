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
	"sync"

	"github.com/golang/glog"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

type Monitor struct {
	cache map[streamKey]map[subscriptionKey]eventingv1alpha1.SubscriptionSpec
	mutex *sync.Mutex
}

func NewMonitor(informerFactory informers.SharedInformerFactory) *Monitor {

	subscriptionInformer := informerFactory.Eventing().V1alpha1().Subscriptions()

	monitor := &Monitor{
		cache: make(map[streamKey]map[subscriptionKey]eventingv1alpha1.SubscriptionSpec),
		mutex: &sync.Mutex{},
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			subscription := obj.(eventingv1alpha1.Subscription)
			monitor.createOrUpdateSubscription(subscription)
		},
		UpdateFunc: func(old, new interface{}) {
			oldSubscription := old.(eventingv1alpha1.Subscription)
			newSubscription := new.(eventingv1alpha1.Subscription)

			if oldSubscription.ResourceVersion == newSubscription.ResourceVersion {
				// Periodic resync will send update events for all known Subscriptions.
				// Two different versions of the same Subscription will always have different RVs.
				return
			}

			monitor.createOrUpdateSubscription(newSubscription)
			if oldSubscription.Spec.Stream != newSubscription.Spec.Stream {
				monitor.removeSubscription(oldSubscription)
			}
		},
		DeleteFunc: func(obj interface{}) {
			subscription := obj.(eventingv1alpha1.Subscription)
			monitor.removeSubscription(subscription)
		},
	})

	return monitor
}

// Subscriptions for a stream name and namespace
func (m *Monitor) Subscriptions(stream string, namespace string) []eventingv1alpha1.SubscriptionSpec {
	streamKey := makeStreamKeyWithNames(stream, namespace)
	subscriptionsCache := m.getOrCreateSubscriptionsCache(streamKey)
	m.mutex.Lock()
	subscriptions := make([]eventingv1alpha1.SubscriptionSpec, len(subscriptionsCache))
	for _, subscription := range subscriptionsCache {
		subscriptions = append(subscriptions, subscription)
	}
	m.mutex.Unlock()
	return subscriptions
}

func (m *Monitor) getOrCreateSubscriptionsCache(key streamKey) map[subscriptionKey]eventingv1alpha1.SubscriptionSpec {
	m.mutex.Lock()
	subscriptionsCache, ok := m.cache[key]
	if !ok {
		subscriptionsCache = make(map[subscriptionKey]eventingv1alpha1.SubscriptionSpec)
		m.cache[key] = subscriptionsCache
	}
	m.mutex.Unlock()
	return subscriptionsCache
}

func (m *Monitor) createOrUpdateSubscription(subscription eventingv1alpha1.Subscription) {
	streamKey := makeStreamKey(subscription)
	subscriptionsCache := m.getOrCreateSubscriptionsCache(streamKey)
	subscriptionKey := makeSubscriptionKey(subscription)
	m.mutex.Lock()
	subscriptionsCache[subscriptionKey] = subscription.Spec
	m.mutex.Unlock()
}

func (m *Monitor) removeSubscription(subscription eventingv1alpha1.Subscription) {
	streamKey := makeStreamKey(subscription)
	subscriptionsCache := m.getOrCreateSubscriptionsCache(streamKey)
	subscriptionKey := makeSubscriptionKey(subscription)
	m.mutex.Lock()
	delete(subscriptionsCache, subscriptionKey)
	m.mutex.Unlock()
}

type streamKey struct {
	Name      string
	Namespace string
}

func makeStreamKey(subscription eventingv1alpha1.Subscription) streamKey {
	return makeStreamKeyWithNames(subscription.Spec.Stream, subscription.Namespace)
}

func makeStreamKeyWithNames(name string, namespace string) streamKey {
	return streamKey{
		Name:      name,
		Namespace: namespace,
	}
}

type subscriptionKey struct {
	Name      string
	Namespace string
}

func makeSubscriptionKey(subscription eventingv1alpha1.Subscription) subscriptionKey {
	return subscriptionKey{
		Name:      subscription.Name,
		Namespace: subscription.Namespace,
	}
}
