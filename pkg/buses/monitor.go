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
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	"github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	channelscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	listers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	Dispatcher  = "dispatcher"
	Provisioner = "provisioner"

	busKind          = "Bus"
	clusterBusKind   = "ClusterBus"
	channelKind      = "Channel"
	subscriptionKind = "Subscription"

	// SuccessSynced is used as part of the Event 'reason' when a resource is synced
	successSynced = "Synced"
	// ErrResourceSync is used as part of the Event 'reason' when a resource fails
	// to sync.
	errResourceSync = "ErrResourceSync"
)

// Monitor is a utility mix-in intended to be used by Bus authors to easily
// write provisioners and dispatchers for buses. Bus provisioners are
// responsible for managing the storage asset(s) that back a channel. Bus
// dispatchers are responsible for dispatching events on the Channel to the
// Channel's Subscriptions. Monitor handles setting up informers that watch a
// Bus, its Channels, and their Subscriptions and allows Bus authors to register
// event handler functions to be called when Provision/Unprovision and
// Subscribe/Unsubscribe happen.
type Monitor struct {
	bus                      channelsv1alpha1.GenericBus
	handler                  MonitorEventHandlerFuncs
	informerFactory          informers.SharedInformerFactory
	busesLister              listers.BusLister
	busesSynced              cache.InformerSynced
	clusterBusesLister       listers.ClusterBusLister
	clusterBusesSynced       cache.InformerSynced
	channelsLister           listers.ChannelLister
	channelsSynced           cache.InformerSynced
	subscriptionsLister      listers.SubscriptionLister
	subscriptionsSynced      cache.InformerSynced
	cache                    map[channelKey]*channelSummary
	provisionedChannels      map[channelKey]*channelsv1alpha1.Channel
	provisionedSubscriptions map[subscriptionKey]*channelsv1alpha1.Subscription
	mutex                    *sync.Mutex

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

type Attributes = map[string]string

// MonitorEventHandlerFuncs is a set of handler functions that are called when a
// bus requires sync, channels are provisioned/unprovisioned, or a subscription
// is created or deleted, or if one of the relevant resources is changed.
type MonitorEventHandlerFuncs struct {
	// BusFunc is invoked when the Bus requires sync.
	BusFunc func(bus channelsv1alpha1.GenericBus) error

	// ProvisionFunc is invoked when a new Channel should be provisioned or when
	// the attributes change.
	ProvisionFunc func(channel *channelsv1alpha1.Channel, attributes Attributes) error

	// UnprovisionFunc in invoked when a Channel should be deleted.
	UnprovisionFunc func(channel *channelsv1alpha1.Channel) error

	// SubscribeFunc is invoked when a new Subscription should be set up or when
	// the attributes change.
	SubscribeFunc func(subscription *channelsv1alpha1.Subscription, attributes Attributes) error

	// UnsubscribeFunc is invoked when a Subscription should be deleted.
	UnsubscribeFunc func(subscription *channelsv1alpha1.Subscription) error
}

func (h MonitorEventHandlerFuncs) onBus(bus channelsv1alpha1.GenericBus, monitor *Monitor) error {
	if h.BusFunc != nil {
		err := h.BusFunc(bus)
		if err != nil {
			monitor.recorder.Eventf(bus, corev1.EventTypeWarning, errResourceSync, "Error syncing Bus: %s", err)
		} else {
			monitor.recorder.Event(bus, corev1.EventTypeNormal, successSynced, "Bus synched successfully")
		}
		return err
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onProvision(channel *channelsv1alpha1.Channel, monitor *Monitor) error {
	if h.ProvisionFunc != nil {
		attributes, err := monitor.channelAttributes(channel.Spec)
		if err != nil {
			return err
		}
		err = h.ProvisionFunc(channel, attributes)
		if err != nil {
			monitor.recorder.Eventf(channel, corev1.EventTypeWarning, errResourceSync, "Error provisoning channel: %s", err)
		} else {
			monitor.recorder.Event(channel, corev1.EventTypeNormal, successSynced, "Channel provisioned successfully")
		}
		return err
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onUnprovision(channel *channelsv1alpha1.Channel, monitor *Monitor) error {
	if h.UnprovisionFunc != nil {
		err := h.UnprovisionFunc(channel)
		if err != nil {
			monitor.recorder.Eventf(channel, corev1.EventTypeWarning, errResourceSync, "Error unprovisioning channel: %s", err)
		} else {
			monitor.recorder.Event(channel, corev1.EventTypeNormal, successSynced, "Channel unprovisioned successfully")
		}
		return err
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onSubscribe(subscription *channelsv1alpha1.Subscription, monitor *Monitor) error {
	if h.SubscribeFunc != nil {
		attributes, err := monitor.subscriptionAttributes(subscription.Spec)
		if err != nil {
			return err
		}
		err = h.SubscribeFunc(subscription, attributes)
		if err != nil {
			monitor.recorder.Eventf(subscription, corev1.EventTypeWarning, errResourceSync, "Error subscribing: %s", err)
		} else {
			monitor.recorder.Event(subscription, corev1.EventTypeNormal, successSynced, "Subscribed successfully")
		}
		return err
	}
	return nil
}

func (h MonitorEventHandlerFuncs) onUnsubscribe(subscription *channelsv1alpha1.Subscription, monitor *Monitor) error {
	if h.UnsubscribeFunc != nil {
		err := h.UnsubscribeFunc(subscription)
		if err != nil {
			monitor.recorder.Eventf(subscription, corev1.EventTypeWarning, errResourceSync, "Error unsubscribing: %s", err)
		} else {
			monitor.recorder.Event(subscription, corev1.EventTypeNormal, successSynced, "Unsubscribed successfully")
		}
		return err
	}
	return nil
}

// channelSummary is a record, for a particular Channel, of that Channel's spec
// and its current subscriptions.
type channelSummary struct {
	Channel       *channelsv1alpha1.ChannelSpec
	Subscriptions map[subscriptionKey]subscriptionSummary
}

// subscriptionSummary is a record of a Subscription's spec that is used as part
// of a channelSummary.
type subscriptionSummary struct {
	Subscription channelsv1alpha1.SubscriptionSpec
}

// NewMonitor creates a monitor for a bus given:
//
// component: the name of the component this monitor should use in created k8s events
// masterURL: the URL of the API server the monitor should communicate with
// kubeconfig: the path of a kubeconfig file to create a client connection to the masterURL with
// handler: a MonitorEventHandlerFuncs with handler functions for the monitor to call
func NewMonitor(
	component, masterURL, kubeconfig string,
	handler MonitorEventHandlerFuncs,
) *Monitor {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building clientset: %s", err.Error())
	}

	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)
	busInformer := informerFactory.Channels().V1alpha1().Buses()
	clusterBusInformer := informerFactory.Channels().V1alpha1().ClusterBuses()
	channelInformer := informerFactory.Channels().V1alpha1().Channels()
	subscriptionInformer := informerFactory.Channels().V1alpha1().Subscriptions()

	// Create event broadcaster
	// Add types to the default Kubernetes Scheme so Events can be logged for the component.
	channelscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: component})

	monitor := &Monitor{
		bus:     nil,
		handler: handler,

		informerFactory:          informerFactory,
		busesLister:              busInformer.Lister(),
		busesSynced:              busInformer.Informer().HasSynced,
		clusterBusesLister:       clusterBusInformer.Lister(),
		clusterBusesSynced:       clusterBusInformer.Informer().HasSynced,
		channelsLister:           channelInformer.Lister(),
		channelsSynced:           channelInformer.Informer().HasSynced,
		subscriptionsLister:      subscriptionInformer.Lister(),
		subscriptionsSynced:      subscriptionInformer.Informer().HasSynced,
		cache:                    make(map[channelKey]*channelSummary),
		provisionedChannels:      make(map[channelKey]*channelsv1alpha1.Channel),
		provisionedSubscriptions: make(map[subscriptionKey]*channelsv1alpha1.Subscription),
		mutex: &sync.Mutex{},

		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Monitor"),
		recorder:  recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Bus resources change
	busInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			bus := obj.(*channelsv1alpha1.Bus)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForBus(bus))
		},
		UpdateFunc: func(old, new interface{}) {
			oldBus := old.(*channelsv1alpha1.Bus)
			newBus := new.(*channelsv1alpha1.Bus)

			if oldBus.ResourceVersion == newBus.ResourceVersion {
				// Periodic resync will send update events for all known Buses.
				// Two different versions of the same Bus will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForBus(newBus))
		},
	})
	// Set up an event handler for when ClusterBus resources change
	clusterBusInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			clusterBus := obj.(*channelsv1alpha1.ClusterBus)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForClusterBus(clusterBus))
		},
		UpdateFunc: func(old, new interface{}) {
			oldClusterBus := old.(*channelsv1alpha1.ClusterBus)
			newClusterBus := new.(*channelsv1alpha1.ClusterBus)

			if oldClusterBus.ResourceVersion == newClusterBus.ResourceVersion {
				// Periodic resync will send update events for all known ClusterBuses.
				// Two different versions of the same ClusterBus will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForClusterBus(newClusterBus))
		},
	})
	// Set up an event handler for when Channel resources change
	channelInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(channel))
		},
		UpdateFunc: func(old, new interface{}) {
			oldChannel := old.(*channelsv1alpha1.Channel)
			newChannel := new.(*channelsv1alpha1.Channel)

			if oldChannel.ResourceVersion == newChannel.ResourceVersion {
				// Periodic resync will send update events for all known Channels.
				// Two different versions of the same Channel will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(newChannel))
		},
		DeleteFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(channel))
		},
	})
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
		},
		UpdateFunc: func(old, new interface{}) {
			oldSubscription := old.(*channelsv1alpha1.Subscription)
			newSubscription := new.(*channelsv1alpha1.Subscription)

			if oldSubscription.ResourceVersion == newSubscription.ResourceVersion {
				// Periodic resync will send update events for all known Subscriptions.
				// Two different versions of the same Subscription will always have different RVs.
				return
			}

			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(newSubscription))
		},
		DeleteFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			monitor.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
		},
	})

	return monitor
}

// Channel returns the provisioned Channel with the given name and namespace, or
// nil if such a Channel hasn't been provisioned.
func (m *Monitor) Channel(name string, namespace string) *channelsv1alpha1.Channel {
	channelKey := makeChannelKeyWithNames(namespace, name)
	if channel, ok := m.provisionedChannels[channelKey]; ok {
		return channel
	}
	return nil
}

// Subscription returns the provisioned Subscription with the given name and
// namespace, or nil if such a Subscription hasn't been provisioned.
func (m *Monitor) Subscription(name string, namespace string) *channelsv1alpha1.Subscription {
	subscriptionKey := makeSubscriptionKeyWithNames(namespace, name)
	if subscription, ok := m.provisionedSubscriptions[subscriptionKey]; ok {
		return subscription
	}
	return nil
}

// Subscriptions returns a slice of SubscriptionSpecs for the Channel with the
// given name and namespace, or nil if the Channel hasn't been provisioned.
func (m *Monitor) Subscriptions(channelName string, namespace string) *[]channelsv1alpha1.SubscriptionSpec {
	channelKey := makeChannelKeyWithNames(namespace, channelName)
	summary := m.getChannelSummary(channelKey)
	channel := m.Channel(channelName, namespace)

	if summary == nil || summary.Channel == nil || channel == nil {
		// the channel is unknown
		return nil
	}

	if !m.bus.BacksChannel(channel) {
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

// channelAttributes resolves the given Channel Arguments and the Bus' Channel
// Parameters, returning an Attributes or an Error.
func (m *Monitor) channelAttributes(channel channelsv1alpha1.ChannelSpec) (Attributes, error) {
	genericBusParameters := m.bus.GetSpec().Parameters
	var parameters *[]channelsv1alpha1.Parameter
	if genericBusParameters != nil {
		parameters = genericBusParameters.Channel
	}
	return m.resolveAttributes(parameters, channel.Arguments)
}

// channelAttributes resolves the given Subscription Arguments and the Bus' Subscription
// Parameters, returning an Attributes or an Error.
func (m *Monitor) subscriptionAttributes(subscription channelsv1alpha1.SubscriptionSpec) (Attributes, error) {
	genericBusParameters := m.bus.GetSpec().Parameters
	var parameters *[]channelsv1alpha1.Parameter
	if genericBusParameters != nil {
		parameters = genericBusParameters.Subscription
	}
	return m.resolveAttributes(parameters, subscription.Arguments)
}

// resolveAttributes resolves a slice of Parameters and a slice of arguments and
// returns an Attributes or an error if there are missing Arguments. Each
// Parameter represents a variable that must be provided by an Argument or
// optionally defaulted if a default value for the Parameter is specified.
// resolveAttributes combines the given arrays of Parameters and Arguments,
// using default values where necessary and returning an error if there are
// missing Arguments.
func (m *Monitor) resolveAttributes(parameters *[]channelsv1alpha1.Parameter, arguments *[]channelsv1alpha1.Argument) (Attributes, error) {
	resolved := make(Attributes)
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
			if _, ok := known[arg.Name]; !ok {
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

func (m *Monitor) RequeueSubscription(subscription *channelsv1alpha1.Subscription) {
	glog.Infof("Requeue subscription %q\n", subscription.Name)
	m.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (m *Monitor) Run(busNamespace, busName string, threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer m.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting monitor")
	go m.informerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, m.busesSynced, m.clusterBusesSynced, m.channelsSynced, m.subscriptionsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	if len(busNamespace) == 0 {
		// monitor is for a ClusterBus
		clusterBus, err := m.clusterBusesLister.Get(busName)
		if err != nil {
			glog.Fatalf("Unknown clusterbus %q", busName)
		}
		m.bus = clusterBus
	} else {
		// monitor is for a namespaced Bus
		bus, err := m.busesLister.Buses(busNamespace).Get(busName)
		if err != nil {
			glog.Fatalf("Unknown bus '%s/%s'", busNamespace, busName)
		}
		m.bus = bus
	}

	glog.Info("Starting workers")
	// Launch workers to process resources
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
		// Run the syncHandler, passing it the name string of the resource to be synced.
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
// converge the two. It then updates the Status block of the resource with the
// current status.
func (m *Monitor) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	kind, namespace, name, err := splitWorkqueueKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	if m.bus == nil && !(kind == busKind || kind == clusterBusKind) {
		// don't attempt to sync until we have seen the bus for this monitor
		return fmt.Errorf("Unknown bus for monitor")
	}

	switch kind {
	case busKind:
		err = m.syncBus(namespace, name)
	case clusterBusKind:
		err = m.syncClusterBus(name)
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
			// nothing to do
			return nil
		}

		return err
	}

	// Sync the Bus
	err = m.createOrUpdateBus(bus)
	if err != nil {
		return err
	}

	return nil
}

func (m *Monitor) syncClusterBus(name string) error {
	// Get the ClusterBus resource with this name
	clusterBus, err := m.clusterBusesLister.Get(name)
	if err != nil {
		// The ClusterBus resource may no longer exist
		if errors.IsNotFound(err) {
			// nothing to do
			return nil
		}

		return err
	}

	// Sync the ClusterBus
	err = m.createOrUpdateClusterBus(clusterBus)
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
	err = m.createOrUpdateChannel(channel)
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
	err = m.createOrUpdateSubscription(subscription)
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

func (m *Monitor) createOrUpdateBus(bus *channelsv1alpha1.Bus) error {
	if bus.Namespace != m.bus.GetObjectMeta().GetNamespace() ||
		bus.Name != m.bus.GetObjectMeta().GetName() {
		// this is not our bus
		return nil
	}

	if !reflect.DeepEqual(m.bus.GetSpec(), bus.Spec) {
		m.bus = bus
		err := m.handler.onBus(bus, m)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Monitor) createOrUpdateClusterBus(clusterBus *channelsv1alpha1.ClusterBus) error {
	if clusterBus.Name != m.bus.GetObjectMeta().GetName() {
		// this is not our clusterbus
		return nil
	}

	if !reflect.DeepEqual(m.bus.GetSpec(), clusterBus.Spec) {
		m.bus = clusterBus
		err := m.handler.onBus(clusterBus, m)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Monitor) createOrUpdateChannel(channel *channelsv1alpha1.Channel) error {
	channelKey := makeChannelKeyFromChannel(channel)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	old := summary.Channel
	new := &channel.Spec
	summary.Channel = new
	m.mutex.Unlock()

	if m.bus.BacksChannel(channel) && !reflect.DeepEqual(old, new) {
		err := m.handler.onProvision(channel, m)
		if err != nil {
			return err
		}
		m.provisionedChannels[channelKey] = channel
	}

	return nil
}

func (m *Monitor) removeChannel(namespace string, name string) error {
	channelKey := makeChannelKeyWithNames(namespace, name)
	channel, ok := m.provisionedChannels[channelKey]
	if !ok {
		return nil
	}

	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	summary.Channel = nil
	m.mutex.Unlock()

	err := m.handler.onUnprovision(channel, m)
	if err != nil {
		return err
	}
	delete(m.provisionedChannels, channelKey)

	return nil
}

func (m *Monitor) isSubscriptionProvisioned(subscription *channelsv1alpha1.Subscription) bool {
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)
	_, ok := m.provisionedSubscriptions[subscriptionKey]
	return ok
}

func (m *Monitor) createOrUpdateSubscription(subscription *channelsv1alpha1.Subscription) error {
	subscriptionKey := makeSubscriptionKeyFromSubscription(subscription)
	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	old := summary.Subscriptions[subscriptionKey]
	new := subscriptionSummary{
		Subscription: subscription.Spec,
	}
	summary.Subscriptions[subscriptionKey] = new
	m.mutex.Unlock()

	channel := m.Channel(subscription.Spec.Channel, subscription.Namespace)
	if channel == nil {
		return fmt.Errorf("unknown channel %q for subscription", subscription.Spec.Channel)
	}
	if !m.bus.BacksChannel(channel) {
		return nil
	}

	if !m.isSubscriptionProvisioned(subscription) || !reflect.DeepEqual(old.Subscription, new.Subscription) {
		err := m.handler.onSubscribe(subscription, m)
		if err != nil {
			return err
		}
		m.provisionedSubscriptions[subscriptionKey] = subscription
	}

	return nil
}

func (m *Monitor) removeSubscription(namespace string, name string) error {
	subscriptionKey := makeSubscriptionKeyWithNames(namespace, name)
	subscription, ok := m.provisionedSubscriptions[subscriptionKey]
	if !ok {
		return nil
	}

	channelKey := makeChannelKeyFromSubscription(subscription)
	summary := m.getOrCreateChannelSummary(channelKey)

	m.mutex.Lock()
	delete(summary.Subscriptions, subscriptionKey)
	m.mutex.Unlock()

	err := m.handler.onUnsubscribe(subscription, m)
	if err != nil {
		return err
	}
	delete(m.provisionedSubscriptions, subscriptionKey)

	return nil
}

type channelKey struct {
	Namespace string
	Name      string
}

func makeChannelKeyFromChannel(channel *channelsv1alpha1.Channel) channelKey {
	return makeChannelKeyWithNames(channel.Namespace, channel.Name)
}

func makeChannelKeyFromSubscription(subscription *channelsv1alpha1.Subscription) channelKey {
	return makeChannelKeyWithNames(subscription.Namespace, subscription.Spec.Channel)
}

func makeChannelKeyWithNames(namespace string, name string) channelKey {
	return channelKey{
		Namespace: namespace,
		Name:      name,
	}
}

type subscriptionKey struct {
	Namespace string
	Name      string
}

func makeSubscriptionKeyFromSubscription(subscription *channelsv1alpha1.Subscription) subscriptionKey {
	return makeSubscriptionKeyWithNames(subscription.Namespace, subscription.Name)
}

func makeSubscriptionKeyWithNames(namespace string, name string) subscriptionKey {
	return subscriptionKey{
		Namespace: namespace,
		Name:      name,
	}
}

func makeWorkqueueKeyForBus(bus *channelsv1alpha1.Bus) string {
	return makeWorkqueueKey(busKind, bus.Namespace, bus.Name)
}

func makeWorkqueueKeyForClusterBus(clusterBus *channelsv1alpha1.ClusterBus) string {
	return makeWorkqueueKey(clusterBusKind, "", clusterBus.Name)
}

func makeWorkqueueKeyForChannel(channel *channelsv1alpha1.Channel) string {
	return makeWorkqueueKey(channelKind, channel.Namespace, channel.Name)
}

func makeWorkqueueKeyForSubscription(subscription *channelsv1alpha1.Subscription) string {
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
