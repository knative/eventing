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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

type Monitor struct {
	brokerName string
	handler    MonitorEventHandlerFuncs

	cache map[streamKey]streamSummary
	mutex *sync.Mutex
}

type MonitorEventHandlerFuncs struct {
	ProvisionFunc   func(stream eventingv1alpha1.Stream)
	UnprovisionFunc func(stream eventingv1alpha1.Stream)
	SubscribeFunc   func(subscription eventingv1alpha1.Subscription)
	UnsubscribeFunc func(subscription eventingv1alpha1.Subscription)
}

type streamSummary struct {
	Stream        *eventingv1alpha1.StreamSpec
	Subscriptions map[subscriptionKey]subscriptionSummary
}

type subscriptionSummary struct {
	Subscription eventingv1alpha1.SubscriptionSpec
}

func NewMonitor(brokerName string, informerFactory informers.SharedInformerFactory, handler MonitorEventHandlerFuncs) *Monitor {

	streamInformer := informerFactory.Eventing().V1alpha1().Streams()
	subscriptionInformer := informerFactory.Eventing().V1alpha1().Subscriptions()

	monitor := &Monitor{
		brokerName: brokerName,
		handler:    handler,

		cache: make(map[streamKey]streamSummary),
		mutex: &sync.Mutex{},
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Stream resources change
	streamInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			stream := obj.(eventingv1alpha1.Stream)
			monitor.createOrUpdateStream(stream)
		},
		UpdateFunc: func(old, new interface{}) {
			oldStream := old.(eventingv1alpha1.Stream)
			newStream := new.(eventingv1alpha1.Stream)

			if oldStream.ResourceVersion == newStream.ResourceVersion {
				// Periodic resync will send update events for all known Streams.
				// Two different versions of the same Stream will always have different RVs.
				return
			}

			monitor.createOrUpdateStream(newStream)
			if oldStream.Spec.Broker != newStream.Spec.Broker {
				monitor.removeStream(oldStream)
			}
		},
		DeleteFunc: func(obj interface{}) {
			stream := obj.(eventingv1alpha1.Stream)
			monitor.removeStream(stream)
		},
	})
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
func (m *Monitor) Subscriptions(stream string, namespace string) *[]eventingv1alpha1.SubscriptionSpec {
	streamKey := makeStreamKeyWithNames(stream, namespace)
	summary := m.getOrCreateStreamSummary(streamKey)

	if summary.Stream.Broker != m.brokerName {
		// the stream is not for this broker
		return nil
	}

	m.mutex.Lock()
	subscriptions := make([]eventingv1alpha1.SubscriptionSpec, len(summary.Subscriptions))
	for _, subscription := range summary.Subscriptions {
		subscriptions = append(subscriptions, subscription.Subscription)
	}
	m.mutex.Unlock()

	return &subscriptions
}

func (m *Monitor) getOrCreateStreamSummary(key streamKey) streamSummary {
	m.mutex.Lock()
	summary, ok := m.cache[key]
	if !ok {
		summary = streamSummary{
			Stream:        nil,
			Subscriptions: make(map[subscriptionKey]subscriptionSummary),
		}
		m.cache[key] = summary
	}
	m.mutex.Unlock()

	return summary
}

func (m *Monitor) createOrUpdateStream(stream eventingv1alpha1.Stream) {
	streamKey := makeStreamKeyFromStream(stream)
	summary := m.getOrCreateStreamSummary(streamKey)

	m.mutex.Lock()
	old := summary.Stream
	new := &stream.Spec
	summary.Stream = new
	m.mutex.Unlock()

	if !reflect.DeepEqual(old, new) {
		m.handler.ProvisionFunc(stream)
	}
}

func (m *Monitor) removeStream(stream eventingv1alpha1.Stream) {
	streamKey := makeStreamKeyFromStream(stream)
	summary := m.getOrCreateStreamSummary(streamKey)

	m.mutex.Lock()
	summary.Stream = nil
	m.mutex.Unlock()

	m.handler.UnprovisionFunc(stream)
}

func (m *Monitor) createOrUpdateSubscription(subscription eventingv1alpha1.Subscription) {
	streamKey := makeStreamKeyFromSubscription(subscription)
	summary := m.getOrCreateStreamSummary(streamKey)
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

func (m *Monitor) removeSubscription(subscription eventingv1alpha1.Subscription) {
	streamKey := makeStreamKeyFromSubscription(subscription)
	summary := m.getOrCreateStreamSummary(streamKey)
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)

	m.mutex.Lock()
	delete(summary.Subscriptions, subscriptionKey)
	m.mutex.Unlock()

	m.handler.UnsubscribeFunc(subscription)
}

type streamKey struct {
	Name      string
	Namespace string
}

func makeStreamKeyFromStream(stream eventingv1alpha1.Stream) streamKey {
	return makeStreamKeyWithNames(stream.Name, stream.Namespace)
}

func makeStreamKeyFromSubscription(subscription eventingv1alpha1.Subscription) streamKey {
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

func makeSubscriptionKeyFromSubscription(subscription eventingv1alpha1.Subscription) subscriptionKey {
	return subscriptionKey{
		Name:      subscription.Name,
		Namespace: subscription.Namespace,
	}
}
