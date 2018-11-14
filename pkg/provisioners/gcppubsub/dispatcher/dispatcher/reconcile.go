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

	"github.com/knative/eventing/pkg/buses"

	"github.com/knative/eventing/pkg/apis/duck/v1alpha1"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/gcppubsub/clusterchannelprovisioner"
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

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	dispatcher    *buses.MessageDispatcher
	reconcileChan chan<- event.GenericEvent

	pubSubClientCreator pubsubutil.PubSubClientCreator

	defaultGcpProject string
	defaultSecret     v1.ObjectReference
	defaultSecretKey  string

	subscriptionsLock sync.Mutex
	subscriptions     map[types.NamespacedName]map[types.NamespacedName]context.CancelFunc
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// TODO: use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		logger.Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling Channel")

	// Modify a copy, not the original.
	c = c.DeepCopy()

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the finalizers
		// regardless of the error.
	}

	if updateStatusErr := util.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("channel", c)).Sugar())

	// We are syncing all the subscribers:

	if c.DeletionTimestamp != nil {
		// We use a finalizer to ensure we stop listening on the subscriptions.
		r.stopAllSubscriptions(ctx, c)
		util.RemoveFinalizer(c, finalizerName)
		return nil
	}

	util.AddFinalizer(c, finalizerName)

	err := r.syncSubscriptions(ctx, c)
	return err
}

func key(c *eventingv1alpha1.Channel) types.NamespacedName {
	return types.NamespacedName{
		Namespace: c.Namespace,
		Name:      c.Name,
	}
}

func subscriptionKey(sub *v1alpha1.ChannelSubscriberSpec) types.NamespacedName {
	return types.NamespacedName{
		Namespace: sub.Ref.Namespace,
		Name:      sub.Ref.Name,
	}
}
func (r *reconciler) stopAllSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel) {
	r.subscriptionsLock.Lock()
	defer r.subscriptionsLock.Unlock()
	r.stopAllSubscriptionsUnderLock(ctx, c)
}

func (r *reconciler) stopAllSubscriptionsUnderLock(ctx context.Context, c *eventingv1alpha1.Channel) {
	channelKey := key(c)
	if subscribers, present := r.subscriptions[channelKey]; present {
		for _, subCancel := range subscribers {
			subCancel()
		}
	}
	delete(r.subscriptions, channelKey)
}

func (r *reconciler) syncSubscriptions(ctx context.Context, c *eventingv1alpha1.Channel) error {
	r.subscriptionsLock.Lock()
	defer r.subscriptionsLock.Unlock()

	subscribers := c.Spec.Subscribable
	if subscribers == nil {
		// There are no subscribers.
		r.stopAllSubscriptionsUnderLock(ctx, c)
		return nil
	}

	// We are going to manipulate subscribers, but don't want it written back, so make a throw away
	// copy.
	subscribers = subscribers.DeepCopy()
	for _, subscriber := range subscribers.Subscribers {
		err := r.createSubscriptionUnderLock(ctx, c, &subscriber)
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

	subsToDelete := map[types.NamespacedName]bool{}
	for sub := range activeSubscribers {
		subsToDelete[sub] = true
	}
	for _, sub := range subscribers.Subscribers {
		delete(subsToDelete, subscriptionKey(&sub))
	}
	for subToDelete := range subsToDelete {
		r.subscriptions[channelKey][subToDelete]()
	}
	return nil
}

func (r *reconciler) createSubscriptionUnderLock(ctx context.Context, c *eventingv1alpha1.Channel, sub *v1alpha1.ChannelSubscriberSpec) error {
	ctxWithCancel, cancelFunc := context.WithCancel(ctx)

	channelKey := key(c)
	if r.subscriptions[channelKey] == nil {
		r.subscriptions[channelKey] = map[types.NamespacedName]context.CancelFunc{}
	}
	subKey := subscriptionKey(sub)
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

	subscription := psc.SubscriptionInProject(pubsubutil.GenerateSubName(sub), gcpProject)
	defaults := buses.DispatchDefaults{
		Namespace: c.Namespace,
	}

	// subscription.Receive blocks, so run it in a goroutine.
	go func() {
		r.logger.Info("subscription.Receive start", zap.Any("sub", sub))
		receiveErr := subscription.Receive(ctxWithCancel, func(ctx context.Context, msg pubsubutil.PubSubMessage) {
			message := &buses.Message{
				Headers: msg.Attributes(),
				Payload: msg.Data(),
			}
			err := r.dispatcher.DispatchMessage(message, sub.SubscriberURI, sub.ReplyURI, defaults)
			if err != nil {
				r.logger.Info("Message dispatch failed", zap.Error(err), zap.String("pubSubMessageId", msg.ID()), zap.Any("sub", sub))
				msg.Nack()
			} else {
				r.logger.Info("Message dispatch succeeded", zap.String("pubSubMessageId", msg.ID()), zap.Any("sub", sub))
				msg.Ack()
			}
		})
		// We want to minimize holding the lock. r.reconcileChan may block, so definitely do not do
		// it under lock. But, to prevent a race condition, we must delete from r.subscriptions
		// before using r.reconcileChan.
		func() {
			r.subscriptionsLock.Lock()
			defer r.subscriptionsLock.Unlock()
			delete(r.subscriptions[channelKey], subKey)
		}()
		r.logger.Info("subscription.Receive stopped", zap.Any("sub", sub))
		if receiveErr != nil {
			r.logger.Error("Error receiving messages", zap.Error(receiveErr), zap.Any("sub", sub))
			r.reconcileChan <- event.GenericEvent{
				Meta:   c.GetObjectMeta(),
				Object: c,
			}
		}
	}()
	return nil
}
