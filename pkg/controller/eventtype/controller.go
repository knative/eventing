/*
Copyright 2018 The Knative Authors

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

package eventtype

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"github.com/knative/eventing/pkg/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	sharedclientset "github.com/knative/pkg/client/clientset/versioned"
	sharedinformers "github.com/knative/pkg/client/informers/externalversions"

	feedv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	"github.com/knative/eventing/pkg/client/listers/feeds/v1alpha1"
	"strings"
	"k8s.io/apimachinery/pkg/util/sets"
)

const eventTypeFinalizerName = "event-type-finalizer"

// Controller is the controller implementation for Channel resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// feedclientset is a clientset for our own API group
	feedclientset clientset.Interface
	// sharedclientset is a clientset for shared API groups
	sharedclientset sharedclientset.Interface

	etLister   v1alpha1.EventTypeLister
	etSynced   cache.InformerSynced
	feedLister v1alpha1.FeedLister
	feedSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a new channel controller
func NewController(
	kubeclientset kubernetes.Interface,
	feedclientset clientset.Interface,
	sharedclientset sharedclientset.Interface,
	_ *rest.Config,
	_ kubeinformers.SharedInformerFactory,
	feedInformerFactory informers.SharedInformerFactory,
	_ sharedinformers.SharedInformerFactory) controller.Interface {

	// obtain references to shared index informers for the Feed and EventType types.
	feedInformer := feedInformerFactory.Feeds().V1alpha1().Feeds()
	etInformer := feedInformerFactory.Feeds().V1alpha1().EventTypes()

	c := &Controller{
		kubeclientset:   kubeclientset,
		feedclientset:   feedclientset,
		sharedclientset: sharedclientset,
		etLister:        etInformer.Lister(),
		etSynced:        etInformer.Informer().HasSynced,
		feedLister:      feedInformer.Lister(),
		feedSynced:      feedInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Channels"),
	}

	glog.Info("Setting up event handlers")
	// Set up an event handler for when EventType resources change.
	etInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Ignore the returned error.
			c.syncEventType(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			// Ignore the returned error.
			c.syncEventType(new)
		},
		DeleteFunc: func(old interface{}) {
			// Ignore the returned error.
			c.handleEventTypeDelete(old)
		},
	})
	// Set up an event handler for when Feed resources change. This controller is really just a
	// finalizer, so we only care about when Feeds no longer use an EventType, at which point we
	// will check if it can be deleted.
	feedInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.handleFeedUpdate,
		DeleteFunc: c.handleFeedDelete,
	})

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting EventType controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.etSynced, c.feedSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Channel resources
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
		// Channel resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing channel '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced channel '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the EventType resource with this namespace/name
	et, err := c.etLister.EventTypes(namespace).Get(name)
	if err != nil {
		// The EventType resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("eventType '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	return c.syncEventType(et)
}

// syncEventType is meant to be used in the Informer's event handler functions. It asserts that obj
// is an EventType and then reconciles it.
func (c *Controller) syncEventType(obj interface{}) error {
	et, ok := obj.(*feedv1alpha1.EventType)
	if !ok {
		return fmt.Errorf("unable to assert *EventType: %v", obj)
	}
	return c.handleEventType(et)
}


func (c *Controller) handleEventType(et *feedv1alpha1.EventType) error {
	if et.ObjectMeta.DeletionTimestamp == nil {
		return c.addEventTypeFinalizer(et)
	} else {
		return c.handleEventTypeDelete(et)
	}
}

func (c *Controller) handleEventTypeDelete(old interface{}) error {
	et := old.(*feedv1alpha1.EventType)
	if et.ObjectMeta.DeletionTimestamp == nil {
		// EventType is not yet marked for deletion.
		return nil
	}
	feeds, err := c.findFeedsUsingEventType(et)
	if err != nil {
		glog.Infof("Unable to find feeds using EventType %s: %v", et.Name, err)
		return err
	}
	if len(feeds) == 0 {
		err = c.removeEventTypeFinalizer(et)
		if err != nil {
			glog.Infof("Unable to remove the %s from the EventType %s: %v", eventTypeFinalizerName, et.Name, err)
			return err
		}
	} else {
		glog.Infof("Cannot remove finalizer from EventType %q, %v Feeds still use it.", et.Name, len(feeds))
		err = c.updateEventTypeStatus(et, feeds)
		if err != nil {
			glog.Infof("Unable to update the status of EventType %s: %v", et.Name, err)
			return err
		}
	}
	return nil
}

func (c *Controller) findFeedsUsingEventType(et *feedv1alpha1.EventType) ([]*feedv1alpha1.Feed, error) {
	allFeeds, err := c.feedclientset.FeedsV1alpha1().Feeds(et.Namespace).List(metav1.ListOptions{})
	if err != nil {
		glog.Errorf("Unable to list the feeds: %v", err)
		return nil, err
	}
	feeds := make([]*feedv1alpha1.Feed, 0, len(allFeeds.Items))

	for _, feed := range allFeeds.Items {
		if et.Name == feed.Spec.Trigger.EventType {
			feeds = append(feeds, &feed)
		}
	}
	return feeds, nil
}

func (c *Controller) addEventTypeFinalizer(et *feedv1alpha1.EventType) error {
	etCopy := et.DeepCopy()
	finalizers := sets.NewString(etCopy.Finalizers...)
	finalizers.Insert(eventTypeFinalizerName)
	if finalizers.Len() == len(etCopy.Finalizers) {
		return nil
	}
	etCopy.Finalizers = finalizers.List()
	_, err := c.feedclientset.FeedsV1alpha1().EventTypes(etCopy.Namespace).Update(etCopy)
	if err != nil {
		glog.Infof("Unable to update EventType %s: %v", etCopy.Name, err)
		return err
	}
	return nil
}

func (c *Controller) removeEventTypeFinalizer(et *feedv1alpha1.EventType) error {
	etCopy := et.DeepCopy()
	finalizers := sets.NewString(etCopy.GetFinalizers()...)
	finalizers.Delete(eventTypeFinalizerName)
	etCopy.ObjectMeta.Finalizers = finalizers.List()
	_, err := c.feedclientset.FeedsV1alpha1().EventTypes(etCopy.Namespace).Update(etCopy)
	if err != nil {
		glog.Infof("Unable to update EventType %s: %v", etCopy.Name, err)
		return err
	}
	return nil
}

// updateEventTypeStatus updates an EventType's status with the Feeds that are pinning the
// EventType.
func (c *Controller) updateEventTypeStatus(et *feedv1alpha1.EventType, feedsStillUsingEventType []*feedv1alpha1.Feed) error {
	etCopy := et.DeepCopy()

	// Filter out the existing InUse condition, if present.
	var newConditions []feedv1alpha1.CommonEventTypeCondition
	for _, condition := range etCopy.Status.Conditions {
		if condition.Type != feedv1alpha1.EventTypeInUse {
			newConditions = append(newConditions, condition)
		}
	}

	// Add the up-to-date InUse condition.
	newConditions = append(newConditions, feedv1alpha1.CommonEventTypeCondition{
		Type:    feedv1alpha1.EventTypeInUse,
		Status:  corev1.ConditionTrue,
		Message: fmt.Sprintf("Still in use by the Feeds: %s", getFeedNames(feedsStillUsingEventType)),
	})

	etCopy.Status.Conditions = newConditions
	_, err := c.feedclientset.FeedsV1alpha1().EventTypes(etCopy.Namespace).Update(etCopy)
	if err != nil {
		glog.Infof("Unable to update the status of EventType %s: %v", etCopy.Name, err)
	}
	return err
}

// getFeedNames generates a single string with the names of all the feeds.
func getFeedNames(feeds []*feedv1alpha1.Feed) string {
	feedNames := make([]string, 0, len(feeds))
	for _, feed := range feeds {
		feedNames = append(feedNames, feed.Name)
	}
	return strings.Join(feedNames, ", ")
}

func (c *Controller) handleFeedUpdate(old, new interface{}) {
	// Did it used to listen to me? If not, then ignore. If yes, then does it still listen to me?
	// If so, then ignore. If not, then, can I be deleted?
	oldFeed := old.(*feedv1alpha1.Feed)
	newFeed := new.(*feedv1alpha1.Feed)
	// TODO: Support ClusterEventTypes as well.
	if oldFeed.Spec.Trigger.EventType != newFeed.Spec.Trigger.EventType {
		// Now that the feed is no longer using the old EventType, handle it to check if it can be deleted.
		c.handleFeedDelete(old)
	}
}

func (c *Controller) handleFeedDelete(old interface{}) {
	oldFeed := old.(*feedv1alpha1.Feed)
	// Now that the feed is no longer using the old EventType, handle it to check if it can be deleted.
	c.syncHandler(oldFeed.Namespace + "/" + oldFeed.Spec.Trigger.EventType)
}
