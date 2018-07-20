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

package flow

import (
	"fmt"
	"log"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimetypes "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	// TODO: Get rid of these, but needed as other controllers use them.
	servingclientset "github.com/knative/serving/pkg/client/clientset/versioned"
	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"

	"github.com/knative/eventing/pkg/controller"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	v1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"

	clientset "github.com/knative/eventing/pkg/client/clientset/versioned"
	flowscheme "github.com/knative/eventing/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	channelListers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	feedListers "github.com/knative/eventing/pkg/client/listers/feeds/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/flows/v1alpha1"
)

const controllerAgentName = "flow-controller"

// TODO: This should come from a configmap
const defaultBusName = "stub"

// What field do we assume Object Reference exports as a resolvable target
const targetFieldName = "domainInternal"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Flow is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Flow
	// is synced successfully
	MessageResourceSynced = "Flow synced successfully"
)

var (
	flowControllerKind = v1alpha1.SchemeGroupVersion.WithKind("Flow")
)

// Controller is the controller implementation for Flow resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	// restConfig is used to create dynamic clients for
	// resolving ObjectReference targets.
	restConfig *rest.Config

	// clientset is a clientset for our own API group
	clientset clientset.Interface

	flowsLister listers.FlowLister
	flowsSynced cache.InformerSynced

	feedsLister feedListers.FeedLister
	feedsSynced cache.InformerSynced

	channelsLister channelListers.ChannelLister
	channelsSynced cache.InformerSynced

	subscriptionsLister channelListers.SubscriptionLister
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

// NewController returns a new flow controller
func NewController(
	kubeclientset kubernetes.Interface,
	clientset clientset.Interface,
	servingclientset servingclientset.Interface,
	restConfig *rest.Config,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	flowsInformerFactory informers.SharedInformerFactory,
	routeInformerFactory servinginformers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Flow types.
	flowInformer := flowsInformerFactory.Flows().V1alpha1()

	// obtain a reference to a shared index informer for the Feed types.
	feedInformer := flowsInformerFactory.Feeds().V1alpha1()

	channelInformer := flowsInformerFactory.Channels().V1alpha1().Channels()

	subscriptionInformer := flowsInformerFactory.Channels().V1alpha1().Subscriptions()

	// Create event broadcaster
	// Add flow-controller types to the default Kubernetes Scheme so Events can be
	// logged for flow-controller types.
	flowscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:       kubeclientset,
		restConfig:          restConfig,
		clientset:           clientset,
		flowsLister:         flowInformer.Flows().Lister(),
		flowsSynced:         flowInformer.Flows().Informer().HasSynced,
		feedsLister:         feedInformer.Feeds().Lister(),
		feedsSynced:         feedInformer.Feeds().Informer().HasSynced,
		channelsLister:      channelInformer.Lister(),
		channelsSynced:      channelInformer.Informer().HasSynced,
		subscriptionsLister: subscriptionInformer.Lister(),
		subscriptionsSynced: subscriptionInformer.Informer().HasSynced,
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Flows"),
		recorder:            recorder,
	}

	glog.Info("Setting up event handlers")

	// Set up an event handler for when Flow resources change
	flowInformer.Flows().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFlow,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFlow(new)
		},
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
	glog.Info("Starting Flow controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for Flow informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.flowsSynced); !ok {
		return fmt.Errorf("failed to wait for Flow caches to sync")
	}

	glog.Info("Waiting for Feed informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.feedsSynced); !ok {
		return fmt.Errorf("failed to wait for Feed caches to sync")
	}

	glog.Info("Waiting for channel informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.channelsSynced); !ok {
		return fmt.Errorf("failed to wait for Channel caches to sync")
	}

	glog.Info("Starting workers")
	// Launch two workers to process Flow resources
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
// attempt to process it, by calling Reconcile.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	if err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		key, ok := obj.(string)
		if !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the Reconcile, passing it the namespace/name string of the
		// Flow resource to be synced.
		if err := c.Reconcile(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj); err != nil {
		runtime.HandleError(err)
	}

	return true
}

// enqueueFlow takes a Flow resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Flow.
func (c *Controller) enqueueFlow(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Flow resource
// with the current status of the resource.
func (c *Controller) Reconcile(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Flow resource with this namespace/name
	original, err := c.flowsLister.Flows(namespace).Get(name)
	if err != nil {
		// The Flow resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("flow '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Don't mutate the informer's copy of our object.
	flow := original.DeepCopy()

	// Reconcile this copy of the Flow and then write back any status
	// updates regardless of whether the reconcile error out.
	err = c.reconcile(flow)
	if equality.Semantic.DeepEqual(original.Status, flow.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(flow); err != nil {
		glog.Warningf("Failed to update flow status: %v", err)
		return err
	}
	return err
}

func (c *Controller) reconcile(flow *v1alpha1.Flow) error {
	// See if the flow has been deleted
	accessor, err := meta.Accessor(flow)
	if err != nil {
		log.Fatalf("Failed to get metadata: %s", err)
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	glog.Infof("DeletionTimestamp: %v", deletionTimestamp)

	target, err := c.resolveActionTarget(flow.Namespace, flow.Spec.Action)
	if err != nil {
		glog.Warningf("Failed to resolve target %v : %v", flow.Spec.Action, err)
		return err
	}

	// Ok, so target is the underlying k8s service (or URI if so specified) that we want to target
	glog.Infof("Resolved Target to: %q", target)

	// Reconcile the Channel. Creates a channel that is the target that the Feed will use.
	// TODO: We should reuse channels possibly.
	channel, err := c.reconcileChannel(flow)
	if err != nil {
		glog.Warningf("Failed to reconcile channel : %v", err)
		return err
	}
	flow.Status.PropagateChannelStatus(channel.Status)

	subscription, err := c.reconcileSubscription(channel.Name, target, flow)
	if err != nil {
		glog.Warningf("Failed to reconcile subscription : %v", err)
		return err
	}
	flow.Status.PropagateSubscriptionStatus(subscription.Status)

	channelDNS := channel.Status.DomainInternal
	if channelDNS != "" {
		glog.Infof("Reconciling feed for flow %q targeting %q", flow.Name, channelDNS)
		feed, err := c.reconcileFeed(channelDNS, flow)
		if err != nil {
			glog.Warningf("Failed to reconcile feed: %v", err)
		}
		flow.Status.PropagateFeedStatus(feed.Status)
	}
	return nil
}

func (c *Controller) updateStatus(u *v1alpha1.Flow) (*v1alpha1.Flow, error) {
	flowClient := c.clientset.FlowsV1alpha1().Flows(u.Namespace)
	newu, err := flowClient.Get(u.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	newu.Status = u.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Flow resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return flowClient.Update(newu)
}

// AddFinalizer adds value to the list of finalizers on obj
func AddFinalizer(obj runtimetypes.Object, value string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	finalizers := sets.NewString(accessor.GetFinalizers()...)
	finalizers.Insert(value)
	accessor.SetFinalizers(finalizers.List())
	return nil
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Flow resource
// with the current status of the resource.
func (c *Controller) resolveActionTarget(namespace string, action v1alpha1.FlowAction) (string, error) {
	glog.Infof("Resolving target: %v", action)

	if action.Target != nil {
		return c.resolveObjectReference(namespace, action.Target)
	}
	if action.TargetURI != nil {
		return *action.TargetURI, nil
	}

	return "", fmt.Errorf("No resolvable action target: %+v", action)
}

// resolveObjectReference fetches an object based on ObjectRefence. It assumes the
// object has a status["domainInternal"] string in it and returns it.
func (c *Controller) resolveObjectReference(namespace string, ref *corev1.ObjectReference) (string, error) {
	resourceClient, err := CreateResourceInterface(c.restConfig, ref, namespace)
	if err != nil {
		glog.Warningf("failed to create dynamic client resource: %v", err)
		return "", err
	}

	obj, err := resourceClient.Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		glog.Warningf("failed to get object: %v", err)
		return "", err
	}
	status, ok := obj.Object["status"]
	if !ok {
		return "", fmt.Errorf("%q does not contain status", ref.Name)
	}
	statusMap := status.(map[string]interface{})
	serviceName, ok := statusMap[targetFieldName]
	if !ok {
		return "", fmt.Errorf("%q does not contain field %q in status", targetFieldName, ref.Name)
	}
	serviceNameStr, ok := serviceName.(string)
	if !ok {
		return "", fmt.Errorf("%q status field %q is not a string", targetFieldName, ref.Name)
	}
	return serviceNameStr, nil
}

func (c *Controller) reconcileChannel(flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	channelName := flow.Name

	channel, err := c.channelsLister.Channels(flow.Namespace).Get(channelName)
	if errors.IsNotFound(err) {
		channel, err = c.createChannel(flow)
		if err != nil {
			glog.Errorf("Failed to create channel %q : %v", channelName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile channel %q failed to get channels : %v", channelName, err)
		return nil, err
	}

	// Should make sure channel is what it should be. For now, just assume it's fine
	// if it exists.
	return channel, err
}

func (c *Controller) createChannel(flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	channelName := flow.Name
	channel := &channelsv1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Name:      channelName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*c.NewControllerRef(flow),
			},
		},
		Spec: channelsv1alpha1.ChannelSpec{
			ClusterBus: defaultBusName,
		},
	}
	return c.clientset.ChannelsV1alpha1().Channels(flow.Namespace).Create(channel)
}

func (c *Controller) reconcileSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscriptionName := flow.Name
	subscription, err := c.subscriptionsLister.Subscriptions(flow.Namespace).Get(subscriptionName)
	if errors.IsNotFound(err) {
		subscription, err = c.createSubscription(channelName, target, flow)
		if err != nil {
			glog.Errorf("Failed to create subscription %q : %v", subscriptionName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile subscription %q failed to get subscriptions : %v", subscriptionName, err)
		return nil, err
	}

	// Should make sure subscription is what it should be. For now, just assume it's fine
	// if it exists.
	return subscription, err
}

func (c *Controller) createSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscriptionName := flow.Name
	subscription := &channelsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      subscriptionName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*c.NewControllerRef(flow),
			},
		},
		Spec: channelsv1alpha1.SubscriptionSpec{
			Channel:    channelName,
			Subscriber: target,
		},
	}
	return c.clientset.ChannelsV1alpha1().Subscriptions(flow.Namespace).Create(subscription)
}

func (c *Controller) reconcileFeed(channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	feedName := flow.Name
	feed, err := c.feedsLister.Feeds(flow.Namespace).Get(feedName)
	if errors.IsNotFound(err) {
		feed, err = c.createFeed(channelDNS, flow)
		if err != nil {
			glog.Errorf("Failed to create feed %q : %v", feedName, err)
			return nil, err
		}
	} else if err != nil {
		glog.Errorf("Failed to reconcile feed %q failed to get feeds : %v", feedName, err)
		return nil, err
	}

	// Should make sure feed is what it should be. For now, just assume it's fine
	// if it exists.
	return feed, err

}

func (c *Controller) createFeed(channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	feedName := flow.Name
	feed := &feedsv1alpha1.Feed{
		ObjectMeta: metav1.ObjectMeta{
			Name:      feedName,
			Namespace: flow.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*c.NewControllerRef(flow),
			},
		},
		Spec: feedsv1alpha1.FeedSpec{
			Action: feedsv1alpha1.FeedAction{DNSName: channelDNS},
			Trigger: feedsv1alpha1.EventTrigger{
				EventType: flow.Spec.Trigger.EventType,
				Resource:  flow.Spec.Trigger.Resource,
				Service:   flow.Spec.Trigger.Service,
			},
		},
	}
	if flow.Spec.ServiceAccountName != "" {
		feed.Spec.ServiceAccountName = flow.Spec.ServiceAccountName
	}

	if flow.Spec.Trigger.Parameters != nil {
		feed.Spec.Trigger.Parameters = flow.Spec.Trigger.Parameters
	}
	if flow.Spec.Trigger.ParametersFrom != nil {
		feed.Spec.Trigger.ParametersFrom = flow.Spec.Trigger.ParametersFrom
	}

	return c.clientset.FeedsV1alpha1().Feeds(flow.Namespace).Create(feed)
}

func (c *Controller) NewControllerRef(flow *v1alpha1.Flow) *metav1.OwnerReference {
	blockOwnerDeletion := false
	isController := false
	revRef := metav1.NewControllerRef(flow, flowControllerKind)
	revRef.BlockOwnerDeletion = &blockOwnerDeletion
	revRef.Controller = &isController
	return revRef
}
