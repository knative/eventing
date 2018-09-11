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
	"strings"
	"time"

	"go.uber.org/zap"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	eventingclientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	eventingscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions"
	channelslisters "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	informercache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

const (
	// Dispatcher manages the data plane for a bus
	Dispatcher = "dispatcher"
	// Provisioner manages the control plane for a bus
	Provisioner = "provisioner"

	busKind          = "Bus"
	clusterBusKind   = "ClusterBus"
	channelKind      = "Channel"
	subscriptionKind = "Subscription"
)

// Reconciler is a utility mix-in intended to be used by Bus authors to easily
// write provisioners and dispatchers for buses. Bus provisioners are
// responsible for managing the storage asset(s) that back a channel. Bus
// dispatchers are responsible for dispatching events on the Channel to the
// Channel's Subscriptions. Reconciler handles setting up informers that watch
// a Bus, its Channels, and their Subscriptions and allows Bus authors to
// register event handler functions to be called when Provision/Unprovision and
// Subscribe/Unsubscribe happen.
type Reconciler struct {
	bus     channelsv1alpha1.GenericBus
	ref     BusReference
	handler EventHandlerFuncs
	cache   *Cache

	eventingInformerFactory eventinginformers.SharedInformerFactory
	eventingClient          eventingclientset.Interface
	busesLister             channelslisters.BusLister
	busesSynced             informercache.InformerSynced
	clusterBusesLister      channelslisters.ClusterBusLister
	clusterBusesSynced      informercache.InformerSynced
	channelsLister          channelslisters.ChannelLister
	channelsSynced          informercache.InformerSynced
	subscriptionsLister     channelslisters.SubscriptionLister
	subscriptionsSynced     informercache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	logger *zap.SugaredLogger
}

// NewReconciler creates a reconciler for a bus given:
//
// component: the name of the component this reconciler should use in created k8s events
// masterURL: the URL of the API server the reconciler should communicate with
// kubeconfig: the path of a kubeconfig file to create a client connection to the masterURL with
// handler: a EventHandlerFuncs with handler functions for the reconciler to call
func NewReconciler(
	ref BusReference,
	component, masterURL, kubeconfig string,
	cache *Cache, handler EventHandlerFuncs,
	logger *zap.SugaredLogger,
) *Reconciler {
	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Fatalf("Error building kubeconfig: %v", err)
	}

	kubeClient, err := kubernetesclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building kubernetes clientset: %v", err)
	}

	eventingClient, err := eventingclientset.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("Error building eventing clientset: %v", err)
	}

	eventingInformerFactory := eventinginformers.NewSharedInformerFactory(eventingClient, time.Second*30)
	busInformer := eventingInformerFactory.Channels().V1alpha1().Buses()
	clusterBusInformer := eventingInformerFactory.Channels().V1alpha1().ClusterBuses()
	channelInformer := eventingInformerFactory.Channels().V1alpha1().Channels()
	subscriptionInformer := eventingInformerFactory.Channels().V1alpha1().Subscriptions()

	// Create event broadcaster
	// Add types to the default Kubernetes Scheme so Events can be logged for the component.
	eventingscheme.AddToScheme(eventingscheme.Scheme)
	logger.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logger.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(eventingscheme.Scheme, corev1.EventSource{Component: component})

	reconciler := &Reconciler{
		bus:     nil,
		ref:     ref,
		handler: handler,
		cache:   cache,

		eventingClient:          eventingClient,
		eventingInformerFactory: eventingInformerFactory,
		busesLister:             busInformer.Lister(),
		busesSynced:             busInformer.Informer().HasSynced,
		clusterBusesLister:      clusterBusInformer.Lister(),
		clusterBusesSynced:      clusterBusInformer.Informer().HasSynced,
		channelsLister:          channelInformer.Lister(),
		channelsSynced:          channelInformer.Informer().HasSynced,
		subscriptionsLister:     subscriptionInformer.Lister(),
		subscriptionsSynced:     subscriptionInformer.Informer().HasSynced,

		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Reconciler"),
		recorder:  recorder,

		logger: logger,
	}

	logger.Info("Setting up event handlers")
	if reconciler.ref.IsNamespaced() {
		// Set up an event handler for when Bus resources change
		busInformer.Informer().AddEventHandler(informercache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				bus := obj.(*channelsv1alpha1.Bus)
				reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForBus(bus))
			},
			UpdateFunc: func(old, new interface{}) {
				oldBus := old.(*channelsv1alpha1.Bus)
				newBus := new.(*channelsv1alpha1.Bus)

				if oldBus.ResourceVersion == newBus.ResourceVersion {
					// Periodic resync will send update events for all known Buses.
					// Two different versions of the same Bus will always have different RVs.
					return
				}

				reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForBus(newBus))
			},
		})
	} else {
		// Set up an event handler for when ClusterBus resources change
		clusterBusInformer.Informer().AddEventHandler(informercache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				clusterBus := obj.(*channelsv1alpha1.ClusterBus)
				reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForClusterBus(clusterBus))
			},
			UpdateFunc: func(old, new interface{}) {
				oldClusterBus := old.(*channelsv1alpha1.ClusterBus)
				newClusterBus := new.(*channelsv1alpha1.ClusterBus)

				if oldClusterBus.ResourceVersion == newClusterBus.ResourceVersion {
					// Periodic resync will send update events for all known ClusterBuses.
					// Two different versions of the same ClusterBus will always have different RVs.
					return
				}

				reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForClusterBus(newClusterBus))
			},
		})
	}
	// Set up an event handler for when Channel resources change
	channelInformer.Informer().AddEventHandler(informercache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(channel))
		},
		UpdateFunc: func(old, new interface{}) {
			oldChannel := old.(*channelsv1alpha1.Channel)
			newChannel := new.(*channelsv1alpha1.Channel)

			if oldChannel.ResourceVersion == newChannel.ResourceVersion {
				// Periodic resync will send update events for all known Channels.
				// Two different versions of the same Channel will always have different RVs.
				return
			}

			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(newChannel))
		},
		DeleteFunc: func(obj interface{}) {
			channel := obj.(*channelsv1alpha1.Channel)
			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForChannel(channel))
		},
	})
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(informercache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
		},
		UpdateFunc: func(old, new interface{}) {
			oldSubscription := old.(*channelsv1alpha1.Subscription)
			newSubscription := new.(*channelsv1alpha1.Subscription)

			if oldSubscription.ResourceVersion == newSubscription.ResourceVersion {
				// Periodic resync will send update events for all known Subscriptions.
				// Two different versions of the same Subscription will always have different RVs.
				return
			}

			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(newSubscription))
		},
		DeleteFunc: func(obj interface{}) {
			subscription := obj.(*channelsv1alpha1.Subscription)
			reconciler.workqueue.AddRateLimited(makeWorkqueueKeyForSubscription(subscription))
		},
	})

	return reconciler
}

// RequeueSubscription will add the Subscription to the workqueue for future
// processing. Reprocessing a Subscription is often used within a dispatcher
// when a long lived receiver is interrupted by an asynchronous error.
func (r *Reconciler) RequeueSubscription(subscription SubscriptionReference) {
	r.logger.Infof("Requeue subscription %q", subscription.String())
	r.workqueue.AddRateLimited(makeWorkqueueKey(subscriptionKind, subscription.Namespace, subscription.Name))
}

// RecordBusEventf creates a new event for the reconciled bus and records it
// with the api server.
func (r *Reconciler) RecordBusEventf(eventtype, reason, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(r.bus, eventtype, reason, messageFmt, args...)
}

// RecordChannelEventf creates a new event for the channel and records it with
// the api server. Attempts to records an event for an unknown channel are
// ignored.
func (r *Reconciler) RecordChannelEventf(ref ChannelReference, eventtype, reason, messageFmt string, args ...interface{}) {
	channel, err := r.cache.Channel(ref)
	if err != nil {
		// TODO handle error
		return
	}
	r.recorder.Eventf(channel, eventtype, reason, messageFmt, args...)
}

// RecordSubscriptionEventf creates a new event for the subscription and
// records it with the api server. Attempts to records an event for an unknown
// subscription are ignored.
func (r *Reconciler) RecordSubscriptionEventf(ref SubscriptionReference, eventtype, reason, messageFmt string, args ...interface{}) {
	subscription, err := r.cache.Subscription(ref)
	if err != nil {
		// TODO handle error
		return
	}
	r.recorder.Eventf(subscription, eventtype, reason, messageFmt, args...)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (r *Reconciler) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer r.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	r.logger.Info("Starting reconciler")
	go r.eventingInformerFactory.Start(stopCh)

	// Wait for the caches to be synced before starting workers
	r.logger.Info("Waiting for informer caches to sync")
	if err := r.WaitForCacheSync(stopCh); err != nil {
		return err
	}

	if r.ref.IsNamespaced() {
		// reconciler is for a namespaced Bus
		bus, err := r.busesLister.Buses(r.ref.Namespace).Get(r.ref.Name)
		if err != nil {
			r.logger.Fatalf("Unknown bus %q: %v", r.ref.String(), err)
		}
		r.bus = bus.DeepCopy()
	} else {
		// reconciler is for a ClusterBus
		clusterBus, err := r.clusterBusesLister.Get(r.ref.Name)
		if err != nil {
			r.logger.Fatalf("Unknown clusterbus %q: %v", r.ref.String(), err)
		}
		r.bus = clusterBus.DeepCopy()
	}

	r.logger.Info("Starting workers")
	// Launch workers to process resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(r.runWorker, time.Second, stopCh)
	}

	r.logger.Info("Started workers")
	<-stopCh
	r.logger.Info("Shutting down workers")

	return nil
}

// WaitForCacheSync blocks returning until the reconciler's informers have
// synchronized. It returns an error if the caches cannot sync.
func (r *Reconciler) WaitForCacheSync(stopCh <-chan struct{}) error {
	var busesSynced informercache.InformerSynced
	// get correct synced reference for bus type
	if r.ref.IsNamespaced() {
		busesSynced = r.busesSynced
	} else {
		busesSynced = r.clusterBusesSynced
	}
	if ok := informercache.WaitForCacheSync(stopCh, busesSynced, r.channelsSynced, r.subscriptionsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (r *Reconciler) runWorker() {
	for r.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (r *Reconciler) processNextWorkItem() bool {
	obj, shutdown := r.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer r.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer r.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form kind/namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			r.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the kind/namespace/name key of the
		// resource to be synced.
		if err := r.syncHandler(key); err != nil {
			r.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing reconciler '%s': %v", key, err)
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		r.workqueue.Forget(obj)
		r.logger.Infof("Successfully synced reconciler '%s'", key)
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
func (r *Reconciler) syncHandler(key string) error {
	// Convert the kind/namespace/name string into a distinct kind, namespace and name
	kind, namespace, name, err := splitWorkqueueKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	switch kind {
	case busKind:
		err = r.syncBus(namespace, name)
	case clusterBusKind:
		err = r.syncClusterBus(name)
	case channelKind:
		err = r.syncChannel(namespace, name)
	case subscriptionKind:
		err = r.syncSubscription(namespace, name)
	default:
		runtime.HandleError(fmt.Errorf("Unknown resource kind %s", kind))
		return nil
	}

	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) syncBus(namespace string, name string) error {
	// Get the Bus resource with this namespace/name
	bus, err := r.busesLister.Buses(namespace).Get(name)
	if err != nil {
		// The Bus resource may no longer exist
		if errors.IsNotFound(err) {
			// nothing to do
			return nil
		}

		return err
	}

	// Sync the Bus
	err = r.createOrUpdateBus(bus.DeepCopy())
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) syncClusterBus(name string) error {
	// Get the ClusterBus resource with this name
	clusterBus, err := r.clusterBusesLister.Get(name)
	if err != nil {
		// The ClusterBus resource may no longer exist
		if errors.IsNotFound(err) {
			// nothing to do
			return nil
		}

		return err
	}

	// Sync the ClusterBus
	err = r.createOrUpdateBus(clusterBus.DeepCopy())
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) syncChannel(namespace string, name string) error {
	// Get the Channel resource with this namespace/name
	channel, err := r.channelsLister.Channels(namespace).Get(name)
	if err != nil {
		// The Channel resource may no longer exist
		if errors.IsNotFound(err) {
			ref := NewChannelReferenceFromNames(name, namespace)
			err = r.removeChannel(ref)
			if err != nil {
				return err
			}
			return nil
		}

		return err
	}

	// Sync the Channel
	err = r.createOrUpdateChannel(channel)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) syncSubscription(namespace string, name string) error {
	// Get the Subscription resource with this namespace/name
	subscription, err := r.subscriptionsLister.Subscriptions(namespace).Get(name)
	if err != nil {
		// The Subscription resource may no longer exist
		if errors.IsNotFound(err) {
			ref := NewSubscriptionReferenceFromNames(name, namespace)
			err = r.removeSubscription(ref)
			if err != nil {
				return err
			}
			return nil
		}

		return err
	}

	// Sync the Subscription
	err = r.createOrUpdateSubscription(subscription)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) createOrUpdateBus(bus channelsv1alpha1.GenericBus) error {
	if r.ref != NewBusReference(bus) {
		// not the bus for this reconciler
		return nil
	}

	// stash the new bus on the reconciler while retaining the old bus. This
	// operation is threadsafe because there is only a single Bus/ClusterBus
	// that is valid for the Reconciler and the workqueue guarantees that it
	// will not emit the same key concurrently. Any bus received is an updated
	// revision of the current bus.
	bus, r.bus = r.bus, bus
	if !equality.Semantic.DeepEqual(r.bus.GetSpec(), bus.GetSpec()) {
		err := r.handler.onBus(r.bus, r)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) createOrUpdateChannel(channel *channelsv1alpha1.Channel) error {
	if !r.bus.BacksChannel(channel) {
		return nil
	}

	r.cache.AddChannel(channel)
	err := r.handler.onProvision(channel, r)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) removeChannel(ref ChannelReference) error {
	channel, err := r.cache.Channel(ref)
	if err != nil {
		// the channel isn't provisioned
		return nil
	}

	err = r.handler.onUnprovision(channel, r)
	if err != nil {
		return err
	}
	r.cache.RemoveChannel(channel)

	return nil
}

func (r *Reconciler) createOrUpdateSubscription(subscription *channelsv1alpha1.Subscription) error {
	ref := NewChannelReferenceFromSubscription(subscription)
	_, err := r.cache.Channel(ref)
	if err != nil {
		// channel is not provisioned, before erring we need to check if the channel is provionable
		channel, errS := r.channelsLister.Channels(ref.Namespace).Get(ref.Name)
		if errS != nil {
			return err
		}
		if !r.bus.BacksChannel(channel) {
			return nil
		}
		return err
	}

	r.cache.AddSubscription(subscription)
	err = r.handler.onSubscribe(subscription, r)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) removeSubscription(ref SubscriptionReference) error {
	subscription, err := r.cache.Subscription(ref)
	if err != nil {
		return nil
	}

	err = r.handler.onUnsubscribe(subscription, r)
	if err != nil {
		return err
	}
	r.cache.RemoveSubscription(subscription)

	return nil
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
