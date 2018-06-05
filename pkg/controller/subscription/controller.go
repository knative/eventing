/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package subscription

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	istiolisters "github.com/knative/eventing/pkg/client/listers/istio/v1alpha2"
	"github.com/knative/eventing/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	subscriptionscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	elainformers "github.com/knative/serving/pkg/client/informers/externalversions"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	istiov1alpha2 "github.com/knative/eventing/pkg/apis/istio/v1alpha2"
)

const controllerAgentName = "subscription-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Subscription is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Subscription fails
	// to sync due to a RouteRule of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a RouteRule already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Subscription"
	// MessageResourceSynced is the message used for an Event fired when a Subscription
	// is synced successfully
	MessageResourceSynced = "Subscription synced successfully"
)

// Controller is the controller implementation for Subscription resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// subscriptionclientset is a clientset for our own API group
	subscriptionclientset clientset.Interface

	routerulesLister    istiolisters.RouteRuleLister
	routerulesSynced    cache.InformerSynced
	channelsLister      listers.ChannelLister
	channelsSynced      cache.InformerSynced
	subscriptionsLister listers.SubscriptionLister
	subscriptionsSynced cache.InformerSynced

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

// NewController returns a new subscription controller
func NewController(
	kubeclientset kubernetes.Interface,
	subscriptionclientset clientset.Interface,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	subscriptionInformerFactory informers.SharedInformerFactory,
	routeInformerFactory elainformers.SharedInformerFactory) controller.Interface {

	// obtain references to shared index informers for the RouteRule and Subscription
	// types.
	routeruleInformer := subscriptionInformerFactory.Config().V1alpha2().RouteRules()
	channelInformer := subscriptionInformerFactory.Eventing().V1alpha1().Channels()
	subscriptionInformer := subscriptionInformerFactory.Eventing().V1alpha1().Subscriptions()

	// Create event broadcaster
	// Add subscription-controller types to the default Kubernetes Scheme so Events can be
	// logged for subscription-controller types.
	subscriptionscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:         kubeclientset,
		subscriptionclientset: subscriptionclientset,
		routerulesLister:      routeruleInformer.Lister(),
		routerulesSynced:      routeruleInformer.Informer().HasSynced,
		channelsLister:        channelInformer.Lister(),
		channelsSynced:        channelInformer.Informer().HasSynced,
		subscriptionsLister:   subscriptionInformer.Lister(),
		subscriptionsSynced:   subscriptionInformer.Informer().HasSynced,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Subscriptions"),
		recorder:              recorder,
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when Subscription resources change
	subscriptionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueSubscription,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueSubscription(new)
		},
	})
	// Set up an event handler for when RouteRule resources change. This
	// handler will lookup the owner of the given RouteRule, and if it is
	// owned by a Subscription resource will enqueue that Subscription resource for
	// processing. This way, we don't need to implement custom logic for
	// handling RouteRule resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	routeruleInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newRouteRule := new.(*istiov1alpha2.RouteRule)
			oldRouteRule := old.(*istiov1alpha2.RouteRule)
			if newRouteRule.ResourceVersion == oldRouteRule.ResourceVersion {
				// Periodic resync will send update events for all known RouteRules.
				// Two different versions of the same RouteRule will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Subscription controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.routerulesSynced, c.subscriptionsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Subscription resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
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
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Subscription resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Subscription resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Subscription resource with this namespace/name
	subscription, err := c.subscriptionsLister.Subscriptions(namespace).Get(name)
	if err != nil {
		// The Subscription resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("subscription '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	channel, err := c.channelsLister.Channels(namespace).Get(subscription.Spec.Channel)

	var buslessRouterule *istiov1alpha2.RouteRule
	if channel.Spec.Bus == "" {
		buslessRouterule, err = c.syncBuslessRouteRule(subscription)
		if err != nil {
			return err
		}
	}

	// Finally, we update the status block of the Subscription resource to reflect the
	// current state of the world
	err = c.updateSubscriptionStatus(subscription, buslessRouterule)
	if err != nil {
		return err
	}

	c.recorder.Event(subscription, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) syncBuslessRouteRule(subscription *eventingv1alpha1.Subscription) (*istiov1alpha2.RouteRule, error) {

	// Get the routerule with the name specified in Subscription.spec
	routeruleName := controller.SubscriptionRouteRuleName(subscription.ObjectMeta.Name)
	routerule, err := c.routerulesLister.RouteRules(subscription.Namespace).Get(routeruleName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		routerule, err = c.subscriptionclientset.ConfigV1alpha2().RouteRules(subscription.Namespace).Create(newRouteRule(subscription))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return nil, err
	}

	// If the RouteRule is not controlled by this Subscription resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(routerule, subscription) {
		msg := fmt.Sprintf(MessageResourceExists, routerule.Name)
		c.recorder.Event(subscription, corev1.EventTypeWarning, ErrResourceExists, msg)
		return nil, fmt.Errorf(msg)
	}

	return routerule, nil
}

func (c *Controller) updateSubscriptionStatus(subscription *eventingv1alpha1.Subscription, buslessRouterule *istiov1alpha2.RouteRule) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	subscriptionCopy := subscription.DeepCopy()
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Subscription resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.subscriptionclientset.EventingV1alpha1().Subscriptions(subscription.Namespace).Update(subscriptionCopy)
	return err
}

// enqueueSubscription takes a Subscription resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Subscription.
func (c *Controller) enqueueSubscription(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Subscription resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Subscription resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Subscription, we should not do anything more
		// with it.
		if ownerRef.Kind != "Subscription" {
			return
		}

		subscription, err := c.subscriptionsLister.Subscriptions(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of subscription '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueSubscription(subscription)
		return
	}
}

// newRouteRule creates a new RouteRule for a Subscription resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Subscription resource that 'owns' it.
func newRouteRule(subscription *eventingv1alpha1.Subscription) *istiov1alpha2.RouteRule {
	labels := map[string]string{
		"subscription": subscription.Name,
	}
	return &istiov1alpha2.RouteRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.SubscriptionRouteRuleName(subscription.ObjectMeta.Name),
			Namespace: subscription.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(subscription, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Subscription",
				}),
			},
		},
		Spec: istiov1alpha2.RouteRuleSpec{
			Destination: istiov1alpha2.IstioService{
				Name: controller.ChannelServiceName(subscription.Spec.Channel),
			},
			Route: []istiov1alpha2.DestinationWeight{
				{
					Destination: istiov1alpha2.IstioService{
						Name: subscription.Spec.Subscriber,
					},
					Weight: 100,
				},
			},
		},
	}
}
