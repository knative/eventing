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

package buses

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	listers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	busKind          = "Bus"
	channelKind      = "Channel"
	subscriptionKind = "Subscription"
)

// Monitor utility to manage channels and subscriptions for a bus
type Monitor struct {
	busName                  string
	handler                  MonitorEventHandlerFuncs
	bus                      *channelsv1alpha1.BusSpec
	busesLister              listers.BusLister
	busesSynced              cache.InformerSynced
	channelsLister           listers.ChannelLister
	channelsSynced           cache.InformerSynced
	subscriptionsLister      listers.SubscriptionLister
	subscriptionsSynced      cache.InformerSynced
	workqueue                workqueue.RateLimitingInterface
	cache                    map[channelKey]*channelSummary
	provisionedChannels      map[resourceKey]channelsv1alpha1.Channel
	provisionedSubscriptions map[resourceKey]channelsv1alpha1.Subscription
	mutex                    *sync.Mutex
}

// MonitorEventHandlerFuncs handler functions for channel and subscription provisioning
type MonitorEventHandlerFuncs struct {
	BusFunc         func(bus channelsv1alpha1.Bus) error
	ProvisionFunc   func(channel channelsv1alpha1.Channel) error
	UnprovisionFunc func(channel channelsv1alpha1.Channel) error
	SubscribeFunc   func(subscription channelsv1alpha1.Subscription) error
	UnsubscribeFunc func(subscription channelsv1alpha1.Subscription) error
}

func (h MonitorEventHandlerFuncs) onBus(bus channelsv1alpha1.Bus) error {
	if h.BusFunc != nil {
		return h.BusFunc(bus)
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onProvision(channel channelsv1alpha1.Channel) error {
	if h.ProvisionFunc != nil {
		return h.ProvisionFunc(channel)
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onUnprovision(channel channelsv1alpha1.Channel) error {
	if h.UnprovisionFunc != nil {
		return h.UnprovisionFunc(channel)
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onSubscribe(subscription channelsv1alpha1.Subscription) error {
	if h.SubscribeFunc != nil {
		return h.SubscribeFunc(subscription)
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onUnsubscribe(subscription channelsv1alpha1.Subscription) error {
	if h.UnsubscribeFunc != nil {
		return h.UnsubscribeFunc(subscription)
	}
	return nil
}

type channelSummary struct {
	Channel       *channelsv1alpha1.ChannelSpec
	Subscriptions map[subscriptionKey]subscriptionSummary
}

type subscriptionSummary struct {
	Subscription channelsv1alpha1.SubscriptionSpec
}

// NewMonitor creates a monitor for a bus
func NewMonitor(busName string, informerFactory informers.SharedInformerFactory, handler MonitorEventHandlerFuncs) *Monitor {

	busInformer := informerFactory.Channels().V1alpha1().Buses()
	channelInformer := informerFactory.Channels().V1alpha1().Channels()
	subscriptionInformer := informerFactory.Channels().V1alpha1().Subscriptions()

	monitor := &Monitor{
		busName: busName,
		handler: handler,

		bus:                      nil,
		busesLister:              busInformer.Lister(),
		busesSynced:              busInformer.Informer().HasSynced,
		channelsLister:           channelInformer.Lister(),
		channelsSynced:           channelInformer.Informer().HasSynced,
		subscriptionsLister:      subscriptionInformer.Lister(),
		subscriptionsSynced:      subscriptionInformer.Informer().HasSynced,
		workqueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Monitor"),
		cache:                    make(map[channelKey]*channelSummary),
		provisionedChannels:      make(map[resourceKey]channelsv1alpha1.Channel),
		provisionedSubscriptions: make(map[resourceKey]channelsv1alpha1.Subscription),
		mutex: &sync.Mutex{},
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Bus resources change
	busInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			bus := obj.(*channelsv1alpha1.Bus)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForBus(*bus))
		},
		UpdateFunc: func(old, new interface{}) {
			oldBus := old.(*channelsv1alpha1.Bus)
			newBus := new.(*channelsv1alpha1.Bus)

			if oldBus.ResourceVersion == newBus.ResourceVersion {
				// Periodic resync will send update events for all known Buses.
				// Two different versions of the same Bus will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForBus(*newBus))
		},
	})
	// Set up an event handler for when Channel resources change
	channelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(*channel))
		},
		UpdateFunc: func(old, new interface{}) {
			oldChannel := old.(*channelsv1alpha1.Channel)
			newChannel := new.(*channelsv1alpha1.Channel)

			if oldChannel.ResourceVersion == newChannel.ResourceVersion {
				// Periodic resync will send update events for all known Channels.
				// Two different versions of the same Channel will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(*newChannel))
		},
		DeleteFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(*channel))
		},
	})
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(*subscription))
		},
		UpdateFunc: func(old, new interface{}) {
			oldSubscription := old.(*channelsv1alpha1.Subscription)
			newSubscription := new.(*channelsv1alpha1.Subscription)

			if oldSubscription.ResourceVersion == newSubscription.ResourceVersion {
				// Periodic resync will send update events for all known Subscriptions.
				// Two different versions of the same Subscription will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(*newSubscription))
		},
		DeleteFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(*subscription))
		},
	})

	return monitor
}

// Channel for a channel name and namespace
func (m *Monitor) Channel(name string, namespace string) *channelsv1alpha1.Channel {
	resourceKey := makeResourceKey(channelKind, namespace, name)
	if channel, ok := m.provisionedChannels[resourceKey]; ok {
		return &channel
	}
	return nil
}

// Subscription for a subscription name and namespace
func (m *Monitor) Subscription(name string, namespace string) *channelsv1alpha1.Subscription {
	resourceKey := makeResourceKey(subscriptionKind, namespace, name)
	if subscription, ok := m.provisionedSubscriptions[resourceKey]; ok {
		return &subscription
	}
	return nil
}

// Subscriptions for a channel name and namespace
func (m *Monitor) Subscriptions(channel string, namespace string) *[]channelsv1alpha1.SubscriptionSpec {
	channelKey := makeChannelKeyWithNames(channel, namespace)
	summary := m.getChannelSummary(channelKey)

	if summary == nil || summary.Channel == nil {
		// the channel is unknown
		return nil
	}

	if summary.Channel.Bus != m.busName {
		// the channel is not for this bus
		return nil
	}

	m.mutex.Lock()
	subscriptions := []channelsv1alpha1.SubscriptionSpec{}
	for _, subscription := range summary.Subscriptions {
		subscriptions = append(subscriptions, subscription.Subscription)
	}
	m.mutex.Unlock()

	return &subscriptions
}

// ChannelParams resolve parameters for a channel
func (m *Monitor) ChannelParams(channel channelsv1alpha1.ChannelSpec) (map[string]string, error) {
	return m.resolveArguments(m.bus.Parameters, channel.Arguments)
}

// SubscriptionParams resolve parameters for a subscription
func (m *Monitor) SubscriptionParams(
	channel channelsv1alpha1.ChannelSpec,
	subscription channelsv1alpha1.SubscriptionSpec,
) (map[string]string, error) {
	return m.resolveArguments(channel.Parameters, subscription.Arguments)
}

func (m *Monitor) resolveArguments(parameters *[]channelsv1alpha1.Parameter, arguments *[]channelsv1alpha1.Argument) (map[string]string, error) {
	resolved := make(map[string]string)
	known := make(map[string]interface{})
	required := make(map[string]interface{})

	// apply parameters
	if parameters != nil {
		for _, param := range *parameters {
			known[param.Name] = true
			if param.Default != nil {
				resolved[param.Name] = *param.Default
			} else {
				required[param.Name] = true
			}
		}
	}
	// apply arguments
	if arguments != nil {
		for _, arg := range *arguments {
			if _, ok := known[arg.Name]; ok {
				// ignore arguments not defined by parameters
				glog.Warningf("Skipping unknown argument: %s\n", arg.Name)
				continue
			}
			delete(required, arg.Name)
			resolved[arg.Name] = arg.Value
		}
	}

	// check for missing arguments
	if len(required) != 0 {
		missing := []string{}
		for name := range required {
			missing = append(missing, name)
		}
		return nil, fmt.Errorf("missing required arguments: %v", missing)
	}

	return resolved, nil
}

func (m *Monitor) RequeueSubscription(subscription channelsv1alpha1.Subscription) {
	glog.Infof("Requeue subscription %q\n", subscription.Name)
	m.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (m *Monitor) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer m.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting monitor")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, m.busesSynced, m.channelsSynced, m.subscriptionsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Bus resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(m.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (m *Monitor) runWorker() {
	for m.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (m *Monitor) processNextWorkItem() bool {
	obj, shutdown := m.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer m.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer m.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			m.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Bus resource to be synced.
		if err := m.syncHandler(key); err != nil {
			m.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing monitor '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		m.workqueue.Forget(obj)
		glog.Infof("Successfully synced monitor '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Bus resource
// with the current status of the resource.
func (m *Monitor) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	kind, namespace, name, err := splitWorkqueueKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	switch kind {
	case busKind:
		err = m.syncBus(namespace, name)
	case channelKind:
		err = m.syncChannel(namespace, name)
	case subscriptionKind:
		err = m.syncSubscription(namespace, name)
	default:
		runtime.HandleError(fmt.Errorf("Unknown resource kind %s", kind))
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func (m *Monitor) syncBus(namespace string, name string) error {
	// Get the Bus resource with this namespace/name
	bus, err := m.busesLister.Buses(namespace).Get(name)
	if err != nil {
		// The Bus resource may no longer exist
		if errors.IsNotFound(err) {
			err = m.removeBus(namespace, name)
			if err != nil {
				return err
			}
			return nil
		}

		return err
	}

	// Sync the Bus
	err = m.createOrUpdateBus(*bus)
	if err != nil {
		return err
	}

	return nil
}

func (m *Monitor) syncChannel(namespace string, name string) error {
	// Get the Channel resource with this namespace/name
	channel, err := m.channelsLister.Channels(namespace).Get(name)
	if err != nil {
		// The Channel resource may no longer exist
		if errors.IsNotFound(err) {
			err = m.removeChannel(namespace, name)
			if err != nil {
				return err
			}
			return nil
		}

		return err
	}

	// Sync the Channel
	err = m.createOrUpdateChannel(*channel)
	if err != nil {
		return err
	}

	return nil
}

func (m *Monitor) syncSubscription(namespace string, name string) error {
	// Get the Subscription resource with this namespace/name
	subscription, err := m.subscriptionsLister.Subscriptions(namespace).Get(name)
	if err != nil {
		// The Subscription resource may no longer exist
		if errors.IsNotFound(err) {
			err = m.removeSubscription(namespace, name)
			if err != nil {
				return err
			}
			return nil
		}

		return err
	}

	// Sync the Subscription
	err = m.createOrUpdateSubscription(*subscription)
	if err != nil {
		return err
	}

	return nil
}

func (m *Monitor) getChannelSummary(key channelKey) *channelSummary {
	return m.cache[key]
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

func (m *Monitor) createOrUpdateBus(bus channelsv1alpha1.Bus) error {
	if bus.Name != m.busName {
		// this is not our bus
		return nil
	}

	if !reflect.DeepEqual(m.bus, bus.Spec) {
		m.bus = &bus.Spec
		err := m.handler.onBus(bus)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Monitor) removeBus(namespace string, name string) error {
	// nothing to do
	return nil
}

func (m *Monitor) isChannelForBus(channel channelsv1alpha1.Channel) bool {
	return channel.Spec.Bus == m.busName
}

func (m *Monitor) createOrUpdateChannel(channel channelsv1alpha1.Channel) error {
	resourceKey := makeResourceKey(channelKind, channel.Namespace, channel.Name)
	channelKey := makeChannelKeyFromChannel(channel)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	old := summary.Channel
	new := &channel.Spec
	summary.Channel = new
	m.mutex.Unlock()

	if m.isChannelForBus(channel) && !reflect.DeepEqual(old, new) {
		err := m.handler.onProvision(channel)
		if err != nil {
			return err
		}
		m.provisionedChannels[resourceKey] = channel
	}

	return nil
}

func (m *Monitor) removeChannel(namespace string, name string) error {
	resourceKey := makeResourceKey(channelKind, namespace, name)
	channel, ok := m.provisionedChannels[resourceKey]
	if !ok {
		return nil
	}

	channelKey := makeChannelKeyFromChannel(channel)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	summary.Channel = nil
	m.mutex.Unlock()

	err := m.handler.onUnprovision(channel)
	if err != nil {
		return err
	}
	delete(m.provisionedChannels, resourceKey)

	return nil
}

func (m *Monitor) isChannelKnown(subscription channelsv1alpha1.Subscription) bool {
	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getChannelSummary(channelKey)
	return summary != nil && summary.Channel != nil
}

func (m *Monitor) isSubscriptionProvisioned(subscription channelsv1alpha1.Subscription) bool {
	resourceKey := makeResourceKey(subscriptionKind, subscription.Namespace, subscription.Name)
	_, ok := m.provisionedSubscriptions[resourceKey]
	return ok
}

func (m *Monitor) isSubscriptionForBus(subscription channelsv1alpha1.Subscription) bool {
	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getChannelSummary(channelKey)
	return summary != nil && summary.Channel != nil && summary.Channel.Bus == m.busName
}

func (m *Monitor) createOrUpdateSubscription(subscription channelsv1alpha1.Subscription) error {
	resourceKey := makeResourceKey(subscriptionKind, subscription.Namespace, subscription.Name)
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

	if !m.isChannelKnown(subscription) {
		return fmt.Errorf("Unknown channel %q for subscription", subscription.Spec.Channel)
	}
	if !m.isSubscriptionForBus(subscription) {
		return nil
	}

	if !m.isSubscriptionProvisioned(subscription) || !reflect.DeepEqual(old.Subscription, new.Subscription) {
		err := m.handler.onSubscribe(subscription)
		if err != nil {
			return err
		}
		m.provisionedSubscriptions[resourceKey] = subscription
	}

	return nil
}

func (m *Monitor) removeSubscription(namespace string, name string) error {
	resourceKey := makeResourceKey(subscriptionKind, namespace, name)
	subscription, ok := m.provisionedSubscriptions[resourceKey]
	if !ok {
		return nil
	}

	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getOrCreateChannelSummary(channelKey)
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)

	m.mutex.Lock()
	delete(summary.Subscriptions, subscriptionKey)
	m.mutex.Unlock()

	err := m.handler.onUnsubscribe(subscription)
	if err != nil {
		return err
	}
	delete(m.provisionedSubscriptions, resourceKey)

	return nil
}

type resourceKey struct {
	Kind      string
	Namespace string
	Name      string
}

func makeResourceKey(kind string, namespace string, name string) resourceKey {
	return resourceKey{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
	}
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

func makeWorkqueueKeyForBus(bus channelsv1alpha1.Bus) string {
	return makeWorkqueueKey(busKind, bus.Namespace, bus.Name)
}

func makeWorkqueueKeyForChannel(channel channelsv1alpha1.Channel) string {
	return makeWorkqueueKey(channelKind, channel.Namespace, channel.Name)
}

func makeWorkqueueKeyForSubscription(subscription channelsv1alpha1.Subscription) string {
	return makeWorkqueueKey(subscriptionKind, subscription.Namespace, subscription.Name)
}

func makeWorkqueueKey(kind string, namespace string, name string) string {
	return fmt.Sprintf("%s/%s/%s", kind, namespace, name)
}

func splitWorkqueueKey(key string) (string, string, string, error) {
	chunks := strings.Split(key, "/")
	if len(chunks) != 3 {
		return "", "", "", fmt.Errorf("Unknown workqueue key %v", key)
	}
	kind := chunks[0]
	namespace := chunks[1]
	name := chunks[2]
	return kind, namespace, name, nil
}
