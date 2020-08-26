/*
Copyright 2020 The Knative Authors

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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"

	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/messaging"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	subscriptionreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/subscription"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
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
	v1ChannelGVK = v1.SchemeGroupVersion.WithKind("Channel")
)

func newChannelWarnEvent(messageFmt string, args ...interface{}) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, channelReferenceFailed, messageFmt, args...)
}

type Reconciler struct {
	// DynamicClientSet allows us to configure pluggable Build objects
	dynamicClientSet dynamic.Interface

	// listers index properties about resources
	subscriptionLister  listers.SubscriptionLister
	channelLister       listers.ChannelLister
	channelableTracker  eventingduck.ListableTracker
	destinationResolver *resolver.URIResolver
	tracker             tracker.Interface
}

// Check that our Reconciler implements Interface
var _ subscriptionreconciler.Interface = (*Reconciler)(nil)

// Check that our Reconciler implements Finalizer
var _ subscriptionreconciler.Finalizer = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
	// Find the channel for this subscription.
	channel, err := r.getChannel(ctx, subscription)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get Spec.Channel or backing channel as Channelable duck type",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		subscription.Status.MarkReferencesResolvedUnknown(channelReferenceFailed, "Failed to get Spec.Channel or backing channel: %s", err)
		return newChannelWarnEvent("Failed to get Spec.Channel or backing channel: %s", err)
	}

	// Make sure all the URI's that are suppose to be in status are up to date.
	if event := r.resolveSubscriptionURIs(ctx, subscription); event != nil {
		return event
	}

	// Sync the resolved subscription into the channel.
	if event := r.syncChannel(ctx, channel, subscription); event != nil {
		return event
	}

	// No channel sync was needed.

	// Check if the channel has the subscription in its status.
	if event := r.checkChannelStatusForSubscription(ctx, channel, subscription); event != nil {
		return event
	}

	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
	channel, err := r.getChannel(ctx, subscription)
	if err != nil {
		// If the channel was deleted (i.e., error == notFound), just return nil so that
		// the subscription's finalizer is removed and the object is gc'ed.
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	// Remove the Subscription from the Channel's subscribers list only if it was actually added in the first place.
	if subscription.Status.IsAddedToChannel() {
		return r.syncChannel(ctx, channel, subscription)
	}
	return nil
}

func (r Reconciler) checkChannelStatusForSubscription(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) pkgreconciler.Event {
	ss, err := r.getSubStatus(ctx, sub, channel)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get subscription status.", zap.Error(err))
		sub.Status.MarkChannelUnknown(subscriptionNotMarkedReadyByChannel, "Failed to get subscription status: %s", err)
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, subscriptionNotMarkedReadyByChannel, err.Error())
	}

	switch ss.Ready {
	case corev1.ConditionTrue:
		sub.Status.MarkChannelReady()
	case corev1.ConditionUnknown:
		sub.Status.MarkChannelUnknown(subscriptionNotMarkedReadyByChannel, "Subscription marked by Channel as Unknown")
	case corev1.ConditionFalse:
		sub.Status.MarkChannelFailed(subscriptionNotMarkedReadyByChannel, "Subscription marked by Channel as False")
	}

	return nil
}

func (r Reconciler) syncChannel(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) pkgreconciler.Event {
	// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
	// the Channel with this information.
	if patched, err := r.syncPhysicalChannel(ctx, sub, channel, false); err != nil {
		logging.FromContext(ctx).Warn("Failed to sync physical Channel", zap.Error(err))
		sub.Status.MarkNotAddedToChannel(physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		return pkgreconciler.NewEvent(corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to synchronize to channel %q: %v", channel.Name, err)
	} else if patched {
		if sub.DeletionTimestamp.IsZero() {
			sub.Status.MarkAddedToChannel()
			return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SubscriberSync", "Subscription was synchronized to channel %q", channel.Name)
		} else {
			return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SubscriberRemoved", "Subscription was removed from channel %q", channel.Name)
		}
	}
	if sub.DeletionTimestamp.IsZero() {
		sub.Status.MarkAddedToChannel()
	}
	return nil
}

func (r *Reconciler) resolveSubscriptionURIs(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
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
	return nil
}

func (r *Reconciler) resolveSubscriber(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
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

func (r *Reconciler) resolveReply(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
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

func (r *Reconciler) resolveDeadLetterSink(ctx context.Context, subscription *v1.Subscription) pkgreconciler.Event {
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
			subscription.Status.PhysicalSubscription.DeadLetterSinkURI = deadLetterSink
		}
	} else {
		subscription.Status.PhysicalSubscription.DeadLetterSinkURI = nil
	}
	return nil
}

func (r *Reconciler) getSubStatus(ctx context.Context, subscription *v1.Subscription, channel *eventingduckv1alpha1.ChannelableCombined) (eventingduckv1.SubscriberStatus, error) {
	if channel.Annotations != nil {
		if channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1beta1" ||
			channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1" {
			return r.getSubStatusV1(subscription, channel)
		}
	}
	return r.getSubStatusV1Alpha1(ctx, subscription, channel)
}

func (r *Reconciler) getSubStatusV1Alpha1(ctx context.Context, subscription *v1.Subscription, channel *eventingduckv1alpha1.ChannelableCombined) (eventingduckv1.SubscriberStatus, error) {
	subscribableStatus := channel.Status.GetSubscribableTypeStatus()

	if subscribableStatus == nil {
		return eventingduckv1.SubscriberStatus{}, fmt.Errorf("channel.Status.SubscribableStatus is nil")
	}
	for _, sub := range subscribableStatus.Subscribers {
		if sub.UID == subscription.GetUID() &&
			sub.ObservedGeneration == subscription.GetGeneration() {
			subv1 := eventingduckv1.SubscriberStatus{}
			sub.ConvertTo(ctx, &subv1)
			return subv1, nil
		}
	}
	return eventingduckv1.SubscriberStatus{}, fmt.Errorf("subscription %q not present in channel %q subscriber's list", subscription.Name, channel.Name)
}

func (r *Reconciler) getSubStatusV1(subscription *v1.Subscription, channel *eventingduckv1alpha1.ChannelableCombined) (eventingduckv1.SubscriberStatus, error) {
	for _, sub := range channel.Status.Subscribers {
		if sub.UID == subscription.GetUID() &&
			sub.ObservedGeneration == subscription.GetGeneration() {
			return eventingduckv1.SubscriberStatus{
				UID:                sub.UID,
				ObservedGeneration: sub.ObservedGeneration,
				Ready:              sub.Ready,
				Message:            sub.Message,
			}, nil
		}
	}
	return eventingduckv1.SubscriberStatus{}, fmt.Errorf("subscription %q not present in channel %q subscriber's list", subscription.Name, channel.Name)
}

func (r *Reconciler) trackAndFetchChannel(ctx context.Context, sub *v1.Subscription, ref corev1.ObjectReference) (runtime.Object, pkgreconciler.Event) {
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

// getChannel fetches the Channel as specified by the Subscriptions spec.Channel
// and verifies it's a channelable (so that we can operate on it via patches).
// If the Channel is a channels.messaging type (hence, it's only a factory for
// underlying channels), fetch and validate the "backing" channel.
func (r *Reconciler) getChannel(ctx context.Context, sub *v1.Subscription) (*eventingduckv1alpha1.ChannelableCombined, pkgreconciler.Event) {
	logging.FromContext(ctx).Info("Getting channel", zap.Any("channel", sub.Spec.Channel))

	// 1. Track the channel pointed by subscription.
	//   a. If channel is a Channel.messaging.knative.dev
	obj, err := r.trackAndFetchChannel(ctx, sub, sub.Spec.Channel)
	if err != nil {
		logging.FromContext(ctx).Warn("failed", zap.Any("channel", sub.Spec.Channel), zap.Error(err))
		return nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	// Test to see if the channel is Channel.messaging because it is going
	// to have a "backing" channel that is what we need to actually operate on
	// as well as keep track of.
	if v1ChannelGVK.Group == gvk.Group && v1ChannelGVK.Kind == gvk.Kind {
		// Track changes on Channel.
		// Ref: https://github.com/knative/eventing/issues/2641
		// NOTE: There is a race condition with using the channelableTracker
		// for Channel when mixed with the usage of channelLister. The
		// channelableTracker has a different cache than the channelLister,
		// when channelLister.Channels is called because the channelableTracker
		// caused an enqueue, the Channels cache my not have had time to
		// re-sync therefore we have to track Channels using a tracker linked
		// to the cache we intend to use to pull the Channel from. This linkage
		// is setup in NewController for r.tracker.
		if err := r.tracker.TrackReference(tracker.Reference{
			APIVersion: "messaging.knative.dev/v1",
			Kind:       "Channel",
			Namespace:  sub.Namespace,
			Name:       sub.Spec.Channel.Name,
		}, sub); err != nil {
			logging.FromContext(ctx).Info("TrackReference for Channel failed", zap.Any("channel", sub.Spec.Channel), zap.Error(err))
			return nil, err
		}

		logging.FromContext(ctx).Debug("fetching backing channel", zap.Any("channel", sub.Spec.Channel))
		// Because the above (trackAndFetchChannel) gives us back a Channelable
		// the status of it will not have the extra bits we need (namely, pointer
		// and status of the actual "backing" channel), we fetch it using typed
		// lister so that we get those bits.
		channel, err := r.channelLister.Channels(sub.Namespace).Get(sub.Spec.Channel.Name)
		if err != nil {
			return nil, err
		}

		if !channel.Status.IsReady() || channel.Status.Channel == nil {
			logging.FromContext(ctx).Warn("backing channel not ready", zap.Any("channel", sub.Spec.Channel), zap.Any("backing channel", channel))
			return nil, fmt.Errorf("channel is not ready.")
		}

		statCh := corev1.ObjectReference{Name: channel.Status.Channel.Name, Namespace: sub.Namespace, Kind: channel.Status.Channel.Kind, APIVersion: channel.Status.Channel.APIVersion}
		obj, err = r.trackAndFetchChannel(ctx, sub, statCh)
		if err != nil {
			return nil, err
		}
	}

	// Now obj is suppposed to be a Channelable, so check it.
	ch, ok := obj.(*eventingduckv1alpha1.ChannelableCombined)
	if !ok {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("channel", sub.Spec.Channel), zap.Error(err))
		return nil, fmt.Errorf("Failed to convert to Channelable Object: %+v", obj)
	}

	return ch.DeepCopy(), nil
}

func isNilOrEmptyDeliveryDeadLetterSink(delivery *eventingduckv1.DeliverySpec) bool {
	return delivery == nil || equality.Semantic.DeepEqual(delivery, &eventingduckv1.DeliverySpec{}) ||
		delivery.DeadLetterSink == nil
}

func isNilOrEmptyDestination(destination *duckv1.Destination) bool {
	return destination == nil || equality.Semantic.DeepEqual(destination, &duckv1.Destination{})
}

func (r *Reconciler) syncPhysicalChannel(ctx context.Context, sub *v1.Subscription, channel *eventingduckv1alpha1.ChannelableCombined, isDeleted bool) (bool, error) {
	logging.FromContext(ctx).Debug("Reconciling physical from Channel", zap.Any("sub", sub))
	if patched, patchErr := r.patchSubscription(ctx, sub.Namespace, channel, sub); patchErr != nil {
		if isDeleted && apierrors.IsNotFound(patchErr) {
			logging.FromContext(ctx).Warn("Could not find Channel", zap.Any("channel", sub.Spec.Channel))
			return false, nil
		}
		return patched, patchErr
	} else {
		return patched, nil
	}
}

func (r *Reconciler) patchSubscription(ctx context.Context, namespace string, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) (bool, error) {
	after := channel.DeepCopy()

	if sub.DeletionTimestamp.IsZero() {
		r.updateChannelAddSubscription(ctx, after, sub)
	} else {
		r.updateChannelRemoveSubscription(ctx, after, sub)
	}

	patch, err := duck.CreateMergePatch(channel, after)
	if err != nil {
		return false, err
	}
	// If there is nothing to patch, we are good, just return.
	// Empty patch is {}, hence we check for that.
	if len(patch) <= 2 {
		return false, nil
	}

	resourceClient, err := eventingduck.ResourceInterface(r.dynamicClientSet, namespace, channel.GroupVersionKind())
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return false, err
	}
	patched, err := resourceClient.Patch(channel.GetName(), types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return false, err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patch", patch), zap.Any("patched", patched))
	return true, nil
}

func (r *Reconciler) updateChannelRemoveSubscription(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	if channel.Annotations != nil {
		if channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1" ||
			channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1beta1" {
			r.updateChannelRemoveSubscriptionV1Beta1(ctx, channel, sub)
			return
		}
	}
	r.updateChannelRemoveSubscriptionV1Alpha1(ctx, channel, sub)
}

func (r *Reconciler) updateChannelRemoveSubscriptionV1Beta1(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	for i, v := range channel.Spec.Subscribers {
		if v.UID == sub.UID {
			channel.Spec.Subscribers = append(
				channel.Spec.Subscribers[:i],
				channel.Spec.Subscribers[i+1:]...)
			return
		}
	}
}

func (r *Reconciler) updateChannelRemoveSubscriptionV1Alpha1(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	if channel.Spec.Subscribable == nil {
		return
	}

	for i, v := range channel.Spec.Subscribable.Subscribers {
		if v.UID == sub.UID {
			channel.Spec.Subscribable.Subscribers = append(
				channel.Spec.Subscribable.Subscribers[:i],
				channel.Spec.Subscribable.Subscribers[i+1:]...)
			return
		}
	}
}

func (r *Reconciler) updateChannelAddSubscription(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	if channel.Annotations != nil {
		if channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1beta1" ||
			channel.Annotations[messaging.SubscribableDuckVersionAnnotation] == "v1" {
			r.updateChannelAddSubscriptionV1Beta1(ctx, channel, sub)
			return
		}
	}
	r.updateChannelAddSubscriptionV1Alpha1(ctx, channel, sub)
}

func (r *Reconciler) updateChannelAddSubscriptionV1Alpha1(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	if channel.Spec.Subscribable == nil {
		channel.Spec.Subscribable = &eventingduckv1alpha1.Subscribable{
			Subscribers: []eventingduckv1alpha1.SubscriberSpec{{
				UID:               sub.UID,
				Generation:        sub.Generation,
				SubscriberURI:     sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:          sub.Status.PhysicalSubscription.ReplyURI,
				DeadLetterSinkURI: sub.Status.PhysicalSubscription.DeadLetterSinkURI,
			}},
		}
		return
	}

	// Look to update subscriber.
	for i, v := range channel.Spec.Subscribable.Subscribers {
		if v.UID == sub.UID {
			channel.Spec.Subscribable.Subscribers[i].Generation = sub.Generation
			channel.Spec.Subscribable.Subscribers[i].SubscriberURI = sub.Status.PhysicalSubscription.SubscriberURI
			channel.Spec.Subscribable.Subscribers[i].ReplyURI = sub.Status.PhysicalSubscription.ReplyURI
			channel.Spec.Subscribable.Subscribers[i].DeadLetterSinkURI = sub.Status.PhysicalSubscription.DeadLetterSinkURI
			return
		}
	}

	// Must not have been found. Add it.
	channel.Spec.Subscribable.Subscribers = append(channel.Spec.Subscribable.Subscribers,
		eventingduckv1alpha1.SubscriberSpec{
			UID:               sub.UID,
			Generation:        sub.Generation,
			SubscriberURI:     sub.Status.PhysicalSubscription.SubscriberURI,
			ReplyURI:          sub.Status.PhysicalSubscription.ReplyURI,
			DeadLetterSinkURI: sub.Status.PhysicalSubscription.DeadLetterSinkURI,
		})
}

func (r *Reconciler) updateChannelAddSubscriptionV1Beta1(ctx context.Context, channel *eventingduckv1alpha1.ChannelableCombined, sub *v1.Subscription) {
	// Look to update subscriber.
	for i, v := range channel.Spec.Subscribers {
		if v.UID == sub.UID {
			channel.Spec.Subscribers[i].Generation = sub.Generation
			channel.Spec.Subscribers[i].SubscriberURI = sub.Status.PhysicalSubscription.SubscriberURI
			channel.Spec.Subscribers[i].ReplyURI = sub.Status.PhysicalSubscription.ReplyURI
			// Only set the deadletter sink if it's not nil. Otherwise we'll just end up patching
			// empty delivery in there.
			if sub.Status.PhysicalSubscription.DeadLetterSinkURI != nil {
				channel.Spec.Subscribers[i].Delivery = &eventingduckv1beta1.DeliverySpec{
					DeadLetterSink: &duckv1.Destination{
						URI: sub.Status.PhysicalSubscription.DeadLetterSinkURI,
					},
				}
			}
			return
		}
	}

	toAdd := eventingduckv1beta1.SubscriberSpec{
		UID:           sub.UID,
		Generation:    sub.Generation,
		SubscriberURI: sub.Status.PhysicalSubscription.SubscriberURI,
		ReplyURI:      sub.Status.PhysicalSubscription.ReplyURI,
	}
	// Only set the deadletter sink if it's not nil. Otherwise we'll just end up patching
	// empty delivery in there.
	if sub.Status.PhysicalSubscription.DeadLetterSinkURI != nil {
		toAdd.Delivery = &eventingduckv1beta1.DeliverySpec{
			DeadLetterSink: &duckv1.Destination{
				URI: sub.Status.PhysicalSubscription.DeadLetterSinkURI,
			},
		}
	}

	// Must not have been found. Add it.
	channel.Spec.Subscribers = append(channel.Spec.Subscribers, toAdd)
}
