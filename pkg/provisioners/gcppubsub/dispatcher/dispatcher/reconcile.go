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

package dispatcher

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/knative/eventing/pkg/provisioners"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/gcppubsub/controller/clusterchannelprovisioner"
	pubsubutil "github.com/knative/eventing/pkg/provisioners/gcppubsub/util"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName
)

type channelName = types.NamespacedName
type subscriptionName = types.NamespacedName
type empty struct{}

// reconciler reconciles Channels with the gcp-pubsub provisioner. It sets up hanging polling for
// every Subscription to any Channel.
type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	// dispatcher is used to make the actual HTTP requests to downstream subscribers.
	dispatcher provisioners.Dispatcher
	// reconcileChan is a Go channel that allows the reconciler to force reconciliation of a Channel.
	reconcileChan chan<- event.GenericEvent

	pubSubClientCreator pubsubutil.PubSubClientCreator

	// Note that for all the default* parameters below, these must be kept in lock-step with the
	// GCP PubSub Dispatcher's reconciler.
	// Eventually, individual Channels should be allowed to specify different projects and secrets,
	// but for now all Channels use the same project and secret.

	// defaultGcpProject is the GCP project ID where PubSub Topics and Subscriptions are created.
	defaultGcpProject string
	// defaultSecret and defaultSecretKey are the K8s Secret and key in that secret that contain a
	// JSON format GCP service account token, see
	// https://cloud.google.com/iam/docs/creating-managing-service-account-keys#iam-service-account-keys-create-gcloud
	defaultSecret    *v1.ObjectReference
	defaultSecretKey string

	subscriptionsLock sync.Mutex
	// subscriptions contains the cancel functions for all hanging PubSub Subscriptions. The cancel
	// function must be called when we no longer want that subscription to be active. Logically it
	// is a map from Channel name to Subscription name to CancelFunc.
	subscriptions map[channelName]map[subscriptionName]context.CancelFunc
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", request)).Sugar())

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		logging.FromContext(ctx).Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	logging.FromContext(ctx).Info("Reconciling Channel")

	// Modify a copy, not the original.
	c = c.DeepCopy()

	requeue, reconcileErr := r.reconcile(loggingWith(ctx, zap.Any("channel", c)), c)
	if reconcileErr != nil {
		logging.FromContext(ctx).Info("Error reconciling Channel", zap.Error(reconcileErr))
		// Note that we do not return the error here, because we want to update the finalizers
		// regardless of the error.
	}

	if err = util.UpdateChannel(ctx, r.client, c); err != nil {
		logging.FromContext(ctx).Info("Error updating Channel Status", zap.Error(err))
		return reconcile.Result{}, err
	}

	return reconcile.Result{
		Requeue: requeue,
	}, reconcileErr
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles gcp-pubsub Channels.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

// reconcile reconciles this Channel so that the real world matches the intended state. The returned
// boolean indicates if this Channel should be immediately requeued for another reconcile loop. The
// returned error indicates an error during reconciliation.
func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) (bool, error) {
	// We are syncing all the subscribers on this Channel. Every subscriber will have a goroutine
	// running in the background polling the GCP PubSub Subscription.

	if c.DeletionTimestamp != nil {
		// We use a finalizer to ensure we stop listening on the GCP PubSub Subscriptions.
		r.stopAllSubscriptions(ctx, c)
		util.RemoveFinalizer(c, finalizerName)
		return false, nil
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before interacting with GCP PubSub.
	if addFinalizerResult := util.AddFinalizer(c, finalizerName); addFinalizerResult == util.FinalizerAdded {
		return true, nil
	}

	err := r.syncSubscriptions(ctx, c)
	return false, err
}

// key creates the first index into reconciler.subscriptions, based on the Channel's name.
func key(c *eventingv1alpha1.Channel) channelName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

// subscriptionKey creates the second index into reconciler.subscriptions, based on the Subscriber's
// name.
func subscriptionKey(sub *v1alpha1.ChannelSubscriberSpec) subscriptionName {
	return types.NamespacedName{
		Namespace: sub.Ref.Namespace,
		Name:      sub.Ref.Name,
	}
}

// stopAllSubscriptions stops listening to all GCP PubSub Subscriptions for the given Channel.
func (r *reconciler) stopAllSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel) {
	r.subscriptionsLock.Lock()
	defer r.subscriptionsLock.Unlock()
	r.stopAllSubscriptionsUnderLock(ctx, c)
}

// stopAllSubscriptionsUnderLock stops listening to all GCP PubSub Subscriptions for the given
// Channel.
// Note that it can only be called if reconciler.subscriptionsLock is held.
func (r *reconciler) stopAllSubscriptionsUnderLock(ctx context.Context, c *eventingv1alpha1.Channel) {
	channelKey := key(c)
	if subscribers, present := r.subscriptions[channelKey]; present {
		for _, subCancel := range subscribers {
			subCancel()
		}
	}
	delete(r.subscriptions, channelKey)
}

// syncSubscriptions ensures all subscribers of the Channel have a background Goroutine that is
// polling the GCP PubSub Subscriptions representing it. It also removes listeners from Subscribers
// that no longer exist.
func (r *reconciler) syncSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel) error {
	r.subscriptionsLock.Lock()
	defer r.subscriptionsLock.Unlock()

	subscribers := c.Spec.Subscribable
	if subscribers == nil {
		// There are no subscribers.
		r.stopAllSubscriptionsUnderLock(ctx, c)
		return nil
	}

	for _, subscriber := range subscribers.Subscribers {
		err := r.createSubscriptionUnderLock(loggingWith(ctx, zap.Any("subscriber", subscriber)), c, &subscriber)
		if err != nil {
			return err
		}
	}

	// Now remove all subscriptions that are no longer present.
	channelKey := key(c)
	activeSubscribers := r.subscriptions[channelKey]
	if len(subscribers.Subscribers) == len(activeSubscribers) {
		return nil
	}

	// subsToDelete is logically a set, not a map (values have no meaning).
	subsToDelete := make(map[subscriptionName]empty, len(activeSubscribers))
	for sub := range activeSubscribers {
		subsToDelete[sub] = empty{}
	}
	for _, sub := range subscribers.Subscribers {
		delete(subsToDelete, subscriptionKey(&sub))
	}
	for subToDelete := range subsToDelete {
		r.subscriptions[channelKey][subToDelete]()
	}
	return nil
}

// createSubscriptionUnderLock starts a background Goroutine for a single subscriber polling its
// GCP PubSub Subscription.
// Note that it can only be called if reconciler.subscriptionsLock is held.
func (r *reconciler) createSubscriptionUnderLock(ctx context.Context, c *eventingv1alpha1.Channel, sub *v1alpha1.ChannelSubscriberSpec) error {
	channelKey := key(c)
	if r.subscriptions[channelKey] == nil {
		r.subscriptions[channelKey] = make(map[subscriptionName]context.CancelFunc)
	}
	subKey := subscriptionKey(sub)
	if r.subscriptions[channelKey][subKey] != nil {
		// There is already a Goroutine watching this subscription.
		return nil
	}
	ctxWithCancel, cancelFunc := context.WithCancel(ctx)
	r.subscriptions[channelKey][subKey] = cancelFunc

	gcpProject := r.defaultGcpProject
	creds, err := pubsubutil.GetCredentials(ctx, r.client, r.defaultSecret, r.defaultSecretKey)
	if err != nil {
		return err
	}
	psc, err := r.pubSubClientCreator(ctxWithCancel, creds, gcpProject)
	if err != nil {
		return err
	}

	// receiveMessageBlocking blocks, so run it in a goroutine.
	go r.receiveMessagesBlocking(ctxWithCancel, c, sub.DeepCopy(), gcpProject, psc)

	return nil
}

// receiveMessagesBlocking receives messages from GCP PubSub, while blocking forever. If the receive
// fails for any reason, then it will instruct the reconciler to process this Channel again via
// reconciler.reconcileChan.
func (r *reconciler) receiveMessagesBlocking(ctxWithCancel context.Context, c *eventingv1alpha1.Channel, sub *v1alpha1.ChannelSubscriberSpec, gcpProject string, psc pubsubutil.PubSubClient) {
	subscription := psc.SubscriptionInProject(pubsubutil.GenerateSubName(sub), gcpProject)
	defaults := provisioners.DispatchDefaults{
		Namespace: c.Namespace,
	}
	channelKey := key(c)
	subKey := subscriptionKey(sub)

	logging.FromContext(ctxWithCancel).Info("subscription.Receive start")
	receiveErr := subscription.Receive(
		ctxWithCancel,
		receiveFunc(logging.FromContext(ctxWithCancel), sub, defaults, r.dispatcher))
	// We want to minimize holding the lock. r.reconcileChan may block, so definitely do not do
	// it under lock. But, to prevent a race condition, we must delete from r.subscriptions
	// before using r.reconcileChan.
	func() {
		r.subscriptionsLock.Lock()
		defer r.subscriptionsLock.Unlock()
		// It is possible that r.stopAllSubscriptions has been called, which has called
		// delete(r.subscriptions, channelKey). If the channel has already been deleted from
		// r.subscriptions, then we don't need to delete anything.
		if subMap, present := r.subscriptions[channelKey]; present {
			delete(subMap, subKey)
		}
	}()

	logging.FromContext(ctxWithCancel).Info("subscription.Receive stopped")
	if receiveErr != nil {
		logging.FromContext(ctxWithCancel).Error("Error receiving messages", zap.Error(receiveErr))
		r.reconcileChan <- event.GenericEvent{
			Meta:   c.GetObjectMeta(),
			Object: c,
		}
	}
}

func receiveFunc(logger *zap.SugaredLogger, sub *v1alpha1.ChannelSubscriberSpec, defaults provisioners.DispatchDefaults, dispatcher provisioners.Dispatcher) func(context.Context, pubsubutil.PubSubMessage) {
	return func(ctx context.Context, msg pubsubutil.PubSubMessage) {
		message := &provisioners.Message{
			Headers: msg.Attributes(),
			Payload: msg.Data(),
		}
		err := dispatcher.DispatchMessage(message, sub.SubscriberURI, sub.ReplyURI, defaults)
		if err != nil {
			logger.Error("Message dispatch failed", zap.Error(err), zap.String("pubSubMessageId", msg.ID()))
			msg.Nack()
		} else {
			logger.Debug("Message dispatch succeeded", zap.String("pubSubMessageId", msg.ID()))
			msg.Ack()
		}
	}
}

func loggingWith(ctx context.Context, fields ...zap.Field) context.Context {
	logger := logging.FromContext(ctx).Desugar()
	return logging.WithLogger(ctx, logger.With(fields...).Sugar())
}
