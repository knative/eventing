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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimetypes "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	// TODO: Get rid of these, but needed as other controllers use them.
	servinginformers "github.com/knative/serving/pkg/client/informers/externalversions"
	"github.com/knative/serving/pkg/logging/logkey"

	"github.com/knative/eventing/pkg/controller"

	channelsv1alpha1 "github.com/knative/eventing/pkg/apis/channels/v1alpha1"
	feedsv1alpha1 "github.com/knative/eventing/pkg/apis/feeds/v1alpha1"
	v1alpha1 "github.com/knative/eventing/pkg/apis/flows/v1alpha1"

	"context"
	informers "github.com/knative/eventing/pkg/client/informers/externalversions"
	channelListers "github.com/knative/eventing/pkg/client/listers/channels/v1alpha1"
	feedListers "github.com/knative/eventing/pkg/client/listers/feeds/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/flows/v1alpha1"
	"github.com/knative/eventing/pkg/controller/flow/resources"
	"github.com/knative/serving/pkg/logging"
	"go.uber.org/zap"
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

// Controller is the controller implementation for Flow resources
type Controller struct {
	*controller.Base

	// restConfig is used to create dynamic clients for
	// resolving ObjectReference targets.
	restConfig *rest.Config

	flowsLister listers.FlowLister
	flowsSynced cache.InformerSynced

	feedsLister feedListers.FeedLister
	feedsSynced cache.InformerSynced

	channelsLister channelListers.ChannelLister
	channelsSynced cache.InformerSynced

	subscriptionsLister channelListers.SubscriptionLister
	subscriptionsSynced cache.InformerSynced
}

// NewController returns a new flow controller
func NewController(
	opt controller.Options,
	restConfig *rest.Config,
	_ kubeinformers.SharedInformerFactory,
	flowsInformerFactory informers.SharedInformerFactory,
	_ servinginformers.SharedInformerFactory) controller.Interface {

	// obtain a reference to a shared index informer for the Flow types.
	flowInformer := flowsInformerFactory.Flows().V1alpha1()

	// obtain a reference to a shared index informer for the Feed types.
	feedInformer := flowsInformerFactory.Feeds().V1alpha1()

	channelInformer := flowsInformerFactory.Channels().V1alpha1().Channels()

	subscriptionInformer := flowsInformerFactory.Channels().V1alpha1().Subscriptions()

	c := &Controller{
		Base: controller.NewBase(opt, controllerAgentName, "Flows"),

		restConfig:          restConfig,
		flowsLister:         flowInformer.Flows().Lister(),
		flowsSynced:         flowInformer.Flows().Informer().HasSynced,
		feedsLister:         feedInformer.Feeds().Lister(),
		feedsSynced:         feedInformer.Feeds().Informer().HasSynced,
		channelsLister:      channelInformer.Lister(),
		channelsSynced:      channelInformer.Informer().HasSynced,
		subscriptionsLister: subscriptionInformer.Lister(),
		subscriptionsSynced: subscriptionInformer.Informer().HasSynced,
	}

	c.Logger.Info("Setting up event handlers")

	// Set up an event handler for when Flow resources change
	flowInformer.Flows().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueFlow,
		UpdateFunc: func(old, new interface{}) {
			c.enqueueFlow(new)
		},
	})

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	return c.RunController(threadiness, stopCh, c.Reconcile, "Flow")

	// TODO(nicholss): is this ok to not wait? the RunController for Base does not wait for cache sync.

	//// Wait for the caches to be synced before starting workers
	//glog.Info("Waiting for Flow informer caches to sync")
	//if ok := cache.WaitForCacheSync(stopCh, c.flowsSynced); !ok {
	//	return fmt.Errorf("failed to wait for Flow caches to sync")
	//}
	//
	//glog.Info("Waiting for Feed informer caches to sync")
	//if ok := cache.WaitForCacheSync(stopCh, c.feedsSynced); !ok {
	//	return fmt.Errorf("failed to wait for Feed caches to sync")
	//}
	//
	//glog.Info("Waiting for channel informer caches to sync")
	//if ok := cache.WaitForCacheSync(stopCh, c.channelsSynced); !ok {
	//	return fmt.Errorf("failed to wait for Channel caches to sync")
	//}
}

// TODO: move this to serving logkey
const (
	// Service is the key used for service name in structured logs
	logkeyFlow = "knative.dev/service"
)

// loggerWithServiceInfo enriches the logs with service name and namespace.
func loggerWithServiceInfo(logger *zap.SugaredLogger, ns string, name string) *zap.SugaredLogger {
	return logger.With(zap.String(logkey.Namespace, ns), zap.String(logkeyFlow, name))
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
	c.WorkQueue.AddRateLimited(key)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Flow resource
// with the current status of the resource.
func (c *Controller) Reconcile(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		//runtime.HandleError(fmt.Errorf("invalid resource key: %s", key)) // TODO(nicholss) ok to delete?
		return nil
	}

	// Wrap our logger with the additional context of the configuration that we are reconciling.
	logger := loggerWithServiceInfo(c.Logger, namespace, name)
	ctx := logging.WithLogger(context.TODO(), logger)

	// Get the Flow resource with this namespace/name
	original, err := c.flowsLister.Flows(namespace).Get(name)
	if err != nil {
		// The Flow resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			logger.Errorf("flow %q in work queue no longer exists", key)
			//runtime.HandleError(fmt.Errorf("flow '%s' in work queue no longer exists", key)) // TODO(nicholss) ok to delete?
			return nil
		}
		return err
	}

	// Don't mutate the informer's copy of our object.
	flow := original.DeepCopy()

	// Reconcile this copy of the Flow and then write back any status
	// updates regardless of whether the reconcile error out.
	err = c.reconcile(ctx, flow)
	if equality.Semantic.DeepEqual(original.Status, flow.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err := c.updateStatus(flow); err != nil {
		logger.Warnf("Failed to update flow status: %v", zap.Error(err))
		return err
	}
	return err
}

func (c *Controller) reconcile(ctx context.Context, flow *v1alpha1.Flow) error {
	logger := logging.FromContext(ctx)

	// See if the flow has been deleted
	accessor, err := meta.Accessor(flow)
	if err != nil {
		logger.Fatalf("Failed to get metadata: %s", zap.Error(err))
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	logger.Infof("DeletionTimestamp: %v", deletionTimestamp)

	target, err := c.resolveActionTarget(ctx, flow.Namespace, flow.Spec.Action)
	if err != nil {
		logger.Warnf("Failed to resolve target %v : %v", flow.Spec.Action, zap.Error(err))
		return err
	}

	// Ok, so target is the underlying k8s service (or URI if so specified) that we want to target
	logger.Infof("Resolved Target to: %q", target)

	// Reconcile the Channel. Creates a channel that is the target that the Feed will use.
	// TODO: We should reuse channels possibly.
	channel, err := c.reconcileChannel(ctx, flow)
	if err != nil {
		logger.Warnf("Failed to reconcile channel : %v", zap.Error(err))
		return err
	}
	flow.Status.PropagateChannelStatus(channel.Status)

	subscription, err := c.reconcileSubscription(channel.Name, target, flow)
	if err != nil {
		logger.Warnf("Failed to reconcile subscription : %v", zap.Error(err))
		return err
	}
	flow.Status.PropagateSubscriptionStatus(subscription.Status)

	channelDNS := channel.Status.DomainInternal
	if channelDNS != "" {
		logger.Infof("Reconciling feed for flow %q targeting %q", flow.Name, channelDNS)
		feed, err := c.reconcileFeed(ctx, channelDNS, flow)
		if err != nil {
			logger.Warnf("Failed to reconcile feed: %v", zap.Error(err))
		}
		flow.Status.PropagateFeedStatus(feed.Status)
	}
	return nil
}

func (c *Controller) updateStatus(u *v1alpha1.Flow) (*v1alpha1.Flow, error) {
	flowClient := c.EventingClientSet.FlowsV1alpha1().Flows(u.Namespace)
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
func (c *Controller) resolveActionTarget(ctx context.Context, namespace string, action v1alpha1.FlowAction) (string, error) {
	logger := logging.FromContext(ctx)

	logger.Infof("Resolving target: %v", action)

	if action.Target != nil {
		return c.resolveObjectReference(ctx, namespace, action.Target)
	}
	if action.TargetURI != nil {
		return *action.TargetURI, nil
	}

	return "", fmt.Errorf("No resolvable action target: %+v", action)
}

// resolveObjectReference fetches an object based on ObjectRefence. It assumes the
// object has a status["domainInternal"] string in it and returns it.
func (c *Controller) resolveObjectReference(ctx context.Context, namespace string, ref *corev1.ObjectReference) (string, error) {
	logger := logging.FromContext(ctx)

	resourceClient, err := CreateResourceInterface(c.restConfig, ref, namespace)
	if err != nil {
		logger.Warnf("failed to create dynamic client resource: %v", zap.Error(err))
		return "", err
	}

	obj, err := resourceClient.Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		c.Logger.Warnf("failed to get object: %v", zap.Error(err))
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

func (c *Controller) reconcileChannel(ctx context.Context, flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	logger := logging.FromContext(ctx)

	channelName := flow.Name

	channel, err := c.channelsLister.Channels(flow.Namespace).Get(channelName)
	if errors.IsNotFound(err) {
		channel, err = c.createChannel(flow)
		if err != nil {
			logger.Errorf("Failed to create channel %q : %v", channelName, zap.Error(err))
			return nil, err
		}
	} else if err != nil {
		logger.Errorf("Failed to reconcile channel %q failed to get channels : %v", channelName, zap.Error(err))
		return nil, err
	}

	// Should make sure channel is what it should be. For now, just assume it's fine
	// if it exists.
	return channel, err
}

func (c *Controller) createChannel(flow *v1alpha1.Flow) (*channelsv1alpha1.Channel, error) {
	channel := resources.MakeChannel(defaultBusName, flow)
	return c.EventingClientSet.ChannelsV1alpha1().Channels(flow.Namespace).Create(channel)
}

func (c *Controller) reconcileSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscriptionName := flow.Name
	subscription, err := c.subscriptionsLister.Subscriptions(flow.Namespace).Get(subscriptionName)
	if errors.IsNotFound(err) {
		subscription, err = c.createSubscription(channelName, target, flow)
		if err != nil {
			c.Logger.Errorf("Failed to create subscription %q : %v", subscriptionName, zap.Error(err))
			return nil, err
		}
	} else if err != nil {
		c.Logger.Errorf("Failed to reconcile subscription %q failed to get subscriptions : %v", subscriptionName, zap.Error(err))
		return nil, err
	}

	// Should make sure subscription is what it should be. For now, just assume it's fine
	// if it exists.
	return subscription, err
}

func (c *Controller) createSubscription(channelName string, target string, flow *v1alpha1.Flow) (*channelsv1alpha1.Subscription, error) {
	subscription := resources.MakeSubscription(channelName, target, flow)
	return c.EventingClientSet.ChannelsV1alpha1().Subscriptions(flow.Namespace).Create(subscription)
}

func (c *Controller) reconcileFeed(ctx context.Context, channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	logger := logging.FromContext(ctx)

	feedName := flow.Name
	feed, err := c.feedsLister.Feeds(flow.Namespace).Get(feedName)
	if errors.IsNotFound(err) {
		feed, err = c.createFeed(channelDNS, flow)
		if err != nil {
			logger.Errorf("Failed to create feed %q : %v", feedName, zap.Error(err))
			return nil, err
		}
	} else if err != nil {
		logger.Errorf("Failed to reconcile feed %q failed to get feeds : %v", feedName, zap.Error(err))
		return nil, err
	}

	// Should make sure feed is what it should be. For now, just assume it's fine
	// if it exists.
	return feed, err

}

func (c *Controller) createFeed(channelDNS string, flow *v1alpha1.Flow) (*feedsv1alpha1.Feed, error) {
	feed := resources.MakeFeed(channelDNS, flow)
	return c.EventingClientSet.FeedsV1alpha1().Feeds(flow.Namespace).Create(feed)
}
