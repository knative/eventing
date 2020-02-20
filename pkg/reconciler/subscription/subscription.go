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

package subscription

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	"sort"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/resolver"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	subscriptionreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1alpha1/subscription"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	subscriptionUpdateStatusFailed      = "UpdateFailed"
	physicalChannelSyncFailed           = "PhysicalChannelSyncFailed"
	subscriptionNotMarkedReadyByChannel = "SubscriptionNotMarkedReadyByChannel"
	channelReferenceFailed              = "ChannelReferenceFailed"
	subscriberResolveFailed             = "SubscriberResolveFailed"
	replyResolveFailed                  = "ReplyResolveFailed"
	deadLetterSinkResolveFailed         = "DeadLetterSinkResolveFailed"
)

var (
	channelGVK = v1alpha1.SchemeGroupVersion.WithKind("Channel")
)

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SubscriptionReconciled", "Subscription reconciled: \"%s/%s\"", namespace, name)
}

func newChannelWarnEvent(messageFmt string, args ...interface{}) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, channelReferenceFailed, messageFmt, args...)
}

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	subscriptionLister  listers.SubscriptionLister
	channelLister       listers.ChannelLister
	channelableTracker  eventingduck.ListableTracker
	destinationResolver *resolver.URIResolver
}

// Check that our Reconciler implements Interface
var _ subscriptionreconciler.Interface = (*Reconciler)(nil)

// Check that our Reconciler implements Finalizer
var _ subscriptionreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, subscription *v1alpha1.Subscription) pkgreconciler.Event {
	subscription.Status.InitializeConditions()
	subscription.Status.ObservedGeneration = subscription.Generation

	// Find the channel for this subscription.
	channel, err := r.getChannel(ctx, subscription)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get Spec.Channel as Channelable duck type",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		subscription.Status.MarkReferencesResolvedUnknown(channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
		return newChannelWarnEvent("Failed to get Spec.Channel as Channelable duck type. %s", err)
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolvedUnknown("Resolving", "Subscription resolution interrupted.")

	if err := r.resolveSubscriber(ctx, subscription); err != nil {
		return err
	}

	if err := r.resolveReply(ctx, subscription); err != nil {
		return err
	}

	if err := r.resolveDeadLetterSink(ctx, subscription); err != nil {
		return err
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Now sync the resolved subscription into the channel.

	if event := r.syncChannel(ctx, channel, subscription); event != nil {
		return event
	}

	// No sync was needed. Time to check the status of the channel to see if the subscription
	// made it to the channel.status.

	ss, err := r.getSubStatusByChannel(subscription, channel)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get subscription status.", zap.Error(err))
		subscription.Status.MarkChannelUnknown(subscriptionNotMarkedReadyByChannel, "Failed to get subscription status: %s", err)
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, subscriptionNotMarkedReadyByChannel, err.Error())
	}
	subStatus := ss.Ready
	if subStatus == corev1.ConditionTrue {
		subscription.Status.MarkChannelReady()
	} else if subStatus == corev1.ConditionUnknown {
		subscription.Status.MarkChannelUnknown(subscriptionNotMarkedReadyByChannel, "Subscription marked by Channel as Unknown")
	} else if subStatus == corev1.ConditionFalse {
		subscription.Status.MarkChannelFailed(subscriptionNotMarkedReadyByChannel, "Subscription marked by Channel as False")
	}

	return newReconciledNormal(subscription.Namespace, subscription.Name)
}

func (r *Reconciler) FinalizeKind(ctx context.Context, subscription *v1alpha1.Subscription) pkgreconciler.Event {
	channel, err := r.getChannel(ctx, subscription)
	if err != nil {
		return err
	}

	return r.syncChannel(ctx, channel, subscription)
}

func (r Reconciler) syncChannel(ctx context.Context, channel *eventingduckv1alpha1.Channelable, sub *v1alpha1.Subscription) pkgreconciler.Event {
	isActive := sub.DeletionTimestamp.IsZero()
	// Check if the subscription needs to be added to the channel
	if isActive != r.subPresentInChannelSpec(sub, channel) {
		// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
		// the Channel with this information.
		if err := r.syncPhysicalChannel(ctx, sub, channel, false); err != nil {
			logging.FromContext(ctx).Warn("Failed to sync physical Channel", zap.Error(err))
			sub.Status.MarkNotAddedToChannel(physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		}
		sub.Status.MarkAddedToChannel()
		return pkgreconciler.NewEvent(corev1.EventTypeNormal, "PhysicalChannelSync", "Subscription was added to channel.")
	}
	if isActive {
		sub.Status.MarkAddedToChannel()
	}
	return nil
}

func (r Reconciler) resolveSubscriber(ctx context.Context, subscription *v1alpha1.Subscription) pkgreconciler.Event {
	// Resolve Subscriber.
	subscriber := subscription.Spec.Subscriber.DeepCopy()
	if !isNilOrEmptyDestination(subscriber) {
		// Populate the namespace for the subscriber since it is in the namespace
		if subscriber.Ref != nil {
			subscriber.Ref.Namespace = subscription.Namespace
		}
		subscriberURI, err := r.destinationResolver.URIFromDestinationV1(*subscriber, subscription)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to resolve Subscriber",
				zap.Error(err),
				zap.Any("subscriber", subscriber))
			subscription.Status.MarkReferencesNotResolved(subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		}
		// If there is a change in resolved URI, log it.
		if subscription.Status.PhysicalSubscription.SubscriberURI == nil || subscription.Status.PhysicalSubscription.SubscriberURI.String() != subscriberURI.String() {
			logging.FromContext(ctx).Debug("Resolved Subscriber", zap.String("subscriberURI", subscriberURI.String()))
			subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
		}
	} else {
		subscription.Status.PhysicalSubscription.SubscriberURI = nil
	}
	return nil
}

func (r Reconciler) resolveReply(ctx context.Context, subscription *v1alpha1.Subscription) pkgreconciler.Event {
	// Resolve Reply.
	reply := subscription.Spec.Reply.DeepCopy()
	if !isNilOrEmptyDestination(reply) {
		// Populate the namespace for the subscriber since it is in the namespace
		if reply.Ref != nil {
			reply.Ref.Namespace = subscription.Namespace
		}
		replyURI, err := r.destinationResolver.URIFromDestinationV1(*reply, subscription)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to resolve reply",
				zap.Error(err),
				zap.Any("reply", reply))
			subscription.Status.MarkReferencesNotResolved(replyResolveFailed, "Failed to resolve spec.reply: %v", err)
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, replyResolveFailed, "Failed to resolve spec.reply: %v", err)
		}
		// If there is a change in resolved URI, log it.
		if subscription.Status.PhysicalSubscription.ReplyURI == nil || subscription.Status.PhysicalSubscription.ReplyURI.String() != replyURI.String() {
			logging.FromContext(ctx).Debug("Resolved reply", zap.String("replyURI", replyURI.String()))
			subscription.Status.PhysicalSubscription.ReplyURI = replyURI
		}
	} else {
		subscription.Status.PhysicalSubscription.ReplyURI = nil
	}
	return nil
}

func (r Reconciler) resolveDeadLetterSink(ctx context.Context, subscription *v1alpha1.Subscription) pkgreconciler.Event {
	// Resolve DeadLetterSink.
	delivery := subscription.Spec.Delivery.DeepCopy()
	if !isNilOrEmptyDeliveryDeadLetterSink(delivery) {
		// Populate the namespace for the dead letter sink since it is in the namespace
		if delivery.DeadLetterSink.Ref != nil {
			delivery.DeadLetterSink.Ref.Namespace = subscription.Namespace
		}

		deadLetterSink, err := r.destinationResolver.URIFromDestinationV1(*delivery.DeadLetterSink, subscription)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to resolve spec.delivery.deadLetterSink",
				zap.Error(err),
				zap.Any("delivery.deadLetterSink", subscription.Spec.Delivery.DeadLetterSink))
			subscription.Status.MarkReferencesNotResolved(deadLetterSinkResolveFailed, "Failed to resolve spec.delivery.deadLetterSink: %v", err)
			return pkgreconciler.NewEvent(corev1.EventTypeWarning, deadLetterSinkResolveFailed, "Failed to resolve spec.delivery.deadLetterSink: %v", err)
		}
		// If there is a change in resolved URI, log it.
		if subscription.Status.PhysicalSubscription.DeadLetterSinkURI == nil || subscription.Status.PhysicalSubscription.DeadLetterSinkURI.String() != deadLetterSink.String() {
			logging.FromContext(ctx).Debug("Resolved deadLetterSink", zap.String("deadLetterSinkURI", deadLetterSink.String()))
			subscription.Status.PhysicalSubscription.ReplyURI = deadLetterSink
		}
	} else {
		subscription.Status.PhysicalSubscription.DeadLetterSinkURI = nil
	}
	return nil
}

func (r Reconciler) getSubStatusByChannel(subscription *v1alpha1.Subscription, channel *eventingduckv1alpha1.Channelable) (eventingduckv1alpha1.SubscriberStatus, error) {
	subscribableStatus := channel.Status.GetSubscribableTypeStatus()

	if subscribableStatus == nil {
		return eventingduckv1alpha1.SubscriberStatus{}, fmt.Errorf("channel.Status.SubscribableStatus is nil")
	}
	for _, sub := range subscribableStatus.Subscribers {
		if sub.UID == subscription.GetUID() &&
			sub.ObservedGeneration == subscription.GetGeneration() {
			return sub, nil
		}
	}
	return eventingduckv1alpha1.SubscriberStatus{}, fmt.Errorf("subscription %q not present in channel %q subscriber's list", subscription.Name, channel.Name)
}

func (r Reconciler) subPresentInChannelSpec(subscription *v1alpha1.Subscription, channel *eventingduckv1alpha1.Channelable) bool {
	if channel.Spec.Subscribable == nil {
		return false
	}
	for _, sub := range channel.Spec.Subscribable.Subscribers {
		if sub.UID == subscription.GetUID() && sub.Generation == subscription.GetGeneration() {
			return true
		}
	}
	return false
}

func (r *Reconciler) trackAndFetchChannel(ctx context.Context, sub *v1alpha1.Subscription, ref corev1.ObjectReference) (runtime.Object, pkgreconciler.Event) {
	// Track the channel using the channelableTracker.
	// We don't need the explicitly set a channelInformer, as this will dynamically generate one for us.
	// This code needs to be called before checking the existence of the `channel`, in order to make sure the
	// subscription controller will reconcile upon a `channel` change.
	if err := r.channelableTracker.TrackInNamespace(sub)(ref); err != nil {
		return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, "TrackerFailed", "unable to track changes to spec.channel: %v", err)
	}
	chLister, err := r.channelableTracker.ListerFor(ref)
	if err != nil {
		logging.FromContext(ctx).Error("Error getting lister for Channel", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	obj, err := chLister.ByNamespace(sub.Namespace).Get(ref.Name)
	if err != nil {
		logging.FromContext(ctx).Error("Error getting channel from lister", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	return obj, err
}

func (r *Reconciler) getChannel(ctx context.Context, sub *v1alpha1.Subscription) (*eventingduckv1alpha1.Channelable, pkgreconciler.Event) {
	// 1. Track the channel pointed by subscription.
	//   a. If channel is a Channel.messaging.knative.dev
	obj, err := r.trackAndFetchChannel(ctx, sub, sub.Spec.Channel)
	if err != nil {
		return nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	// Test to see if the channel is special.
	// NOTE: do not test for version.
	if channelGVK.Group == gvk.Group && channelGVK.Kind == gvk.Kind {
		channel, err := r.channelLister.Channels(sub.Namespace).Get(sub.Spec.Channel.Name)
		if err != nil {
			return nil, err
		}

		if !channel.Status.IsReady() || channel.Status.Channel == nil {
			logging.FromContext(ctx).Warn("channel not ready", zap.Any("channel", sub.Spec.Channel))
			return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, "ChannelNotReady", "Channel is not ready")
		}

		obj, err = r.trackAndFetchChannel(ctx, sub, *channel.Status.Channel)
		if err != nil {
			return nil, err
		}
	}

	// Now ch is supppose to be a Channelable.
	ch, ok := obj.(*eventingduckv1alpha1.Channelable)
	if !ok {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("channel", sub.Spec.Channel), zap.Error(err))
		return nil, err
	}
	return ch, nil
}

func isNilOrEmptyDeliveryDeadLetterSink(delivery *eventingduckv1alpha1.DeliverySpec) bool {
	return delivery == nil || equality.Semantic.DeepEqual(delivery, &eventingduckv1alpha1.DeliverySpec{}) ||
		delivery.DeadLetterSink == nil
}

func isNilOrEmptyDestination(destination *duckv1.Destination) bool {
	return destination == nil || equality.Semantic.DeepEqual(destination, &duckv1.Destination{})
}

func (r *Reconciler) syncPhysicalChannel(ctx context.Context, sub *v1alpha1.Subscription, channel *eventingduckv1alpha1.Channelable, isDeleted bool) error {
	logging.FromContext(ctx).Debug("Reconciling physical from Channel", zap.Any("sub", sub))

	subs, err := r.listAllSubscriptionsWithPhysicalChannel(ctx, sub)
	if err != nil {
		logging.FromContext(ctx).Info("Unable to list all Subscriptions with physical Channel", zap.Error(err))
		return err
	}

	if !isDeleted {
		// The sub we are currently reconciling has not yet written any updated status, so when listing
		// it won't show any updates to the Status.PhysicalSubscription. We know that we are listing
		// for subscriptions with the same PhysicalSubscription.From, so just add this one manually.
		subs = append(subs, *sub)
	}
	subscribable := r.createSubscribable(subs)

	if patchErr := r.patchPhysicalFrom(ctx, sub.Namespace, channel, subscribable); patchErr != nil {
		if isDeleted && apierrors.IsNotFound(patchErr) {
			logging.FromContext(ctx).Warn("Could not find Channel", zap.Any("channel", sub.Spec.Channel))
			return nil
		}
		return patchErr
	}
	return nil
}

func (r *Reconciler) listAllSubscriptionsWithPhysicalChannel(ctx context.Context, sub *v1alpha1.Subscription) ([]v1alpha1.Subscription, error) {
	subs := make([]v1alpha1.Subscription, 0)

	sl, err := r.subscriptionLister.Subscriptions(sub.Namespace).List(labels.Everything()) // TODO: we can use labels to help here
	if err != nil {
		return nil, err
	}
	for _, s := range sl {
		if sub.UID == s.UID {
			// This is the sub that is being reconciled. Skip it.
			continue
		}
		if equality.Semantic.DeepEqual(sub.Spec.Channel, s.Spec.Channel) {
			subs = append(subs, *s)
		}
	}
	return subs, nil
}

func (r *Reconciler) createSubscribable(subs []v1alpha1.Subscription) *eventingduckv1alpha1.Subscribable {
	rv := &eventingduckv1alpha1.Subscribable{}
	// Strictly order the subscriptions, so that simple ordering changes do not cause re-reconciles.
	sort.Slice(subs, func(i, j int) bool {
		return subs[i].UID < subs[j].UID
	})
	for _, sub := range subs {
		if sub.Status.AreReferencesResolved() && sub.DeletionTimestamp == nil {
			rv.Subscribers = append(rv.Subscribers, eventingduckv1alpha1.SubscriberSpec{
				UID:               sub.UID,
				Generation:        sub.Generation,
				SubscriberURI:     sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:          sub.Status.PhysicalSubscription.ReplyURI,
				DeadLetterSinkURI: sub.Status.PhysicalSubscription.DeadLetterSinkURI,
			})
		}
	}
	return rv
}

func (r *Reconciler) patchPhysicalFrom(ctx context.Context, namespace string, origChannel *eventingduckv1alpha1.Channelable, subs *eventingduckv1alpha1.Subscribable) error {
	after := origChannel.DeepCopy()
	after.Spec.Subscribable = subs

	patch, err := duck.CreateMergePatch(origChannel, after)

	if err != nil {
		return err
	}

	resourceClient, err := eventingduck.ResourceInterface(r.DynamicClientSet, namespace, origChannel.GroupVersionKind())
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return err
	}
	patched, err := resourceClient.Patch(origChannel.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patched", patched))
	return nil
}
