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
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionslisters "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/controller"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	eventingduck "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
)

const (
	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	subscriptionReconciled              = "SubscriptionReconciled"
	subscriptionReadinessChanged        = "SubscriptionReadinessChanged"
	subscriptionUpdateStatusFailed      = "SubscriptionUpdateStatusFailed"
	physicalChannelSyncFailed           = "PhysicalChannelSyncFailed"
	subscriptionNotMarkedReadyByChannel = "SubscriptionNotMarkedReadyByChannel"
	channelReferenceFailed              = "ChannelReferenceFailed"
	subscriberResolveFailed             = "SubscriberResolveFailed"
	replyResolveFailed                  = "ReplyResolveFailed"

	// Label to specify valid subscribable channel CRDs.
	channelCrdLabelKey   = "messaging.knative.dev/subscribable"
	channelCrdLabelValue = "true"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	subscriptionLister             listers.SubscriptionLister
	customResourceDefinitionLister apiextensionslisters.CustomResourceDefinitionLister
	resourceTracker                eventingduck.ResourceTracker
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Subscription resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Subscription resource with this namespace/name
	original, err := r.subscriptionLister.Subscriptions(namespace).Get(name)
	if errors.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("subscription key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	subscription := original.DeepCopy()

	// Reconcile this copy of the Subscription and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, subscription)
	if reconcileErr != nil {
		logging.FromContext(ctx).Warn("Error reconciling Subscription", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Subscription reconciled")
		r.Recorder.Eventf(subscription, corev1.EventTypeNormal, subscriptionReconciled, "Subscription reconciled: %q", subscription.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, subscription.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the Subscription", zap.Error(updateStatusErr))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, subscriptionUpdateStatusFailed, "Failed to update Subscription's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, subscription *v1alpha1.Subscription) error {
	subscription.Status.InitializeConditions()

	// Verify subscription is valid.
	if err := subscription.Validate(ctx); err != nil {
		return err
	}

	// Track the channel using the resourceTracker.
	// We don't need the explicitly set a channelInformer, as this will dynamically generate one for us.
	// This code needs to be called before checking the existence of the `channel`, in order to make sure the
	// subscription controller will reconcile upon a `channel` change.
	track := r.resourceTracker.TrackInNamespace(subscription)
	if err := track(subscription.Spec.Channel); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to spec.channel", zap.Error(err))
	}
	if subscription.DeletionTimestamp != nil {

		// If the subscription is Ready, then we have to remove it
		// from the channel's subscriber list.
		if channel, err := r.getChannelable(ctx, subscription.Namespace, &subscription.Spec.Channel); !errors.IsNotFound(err) && subscription.Status.IsAddedToChannel() {
			if err != nil {
				logging.FromContext(ctx).Warn("Failed to get Spec.Channel as Channelable duck type",
					zap.Error(err),
					zap.Any("channel", subscription.Spec.Channel))
				r.Recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
				subscription.Status.MarkReferencesNotResolved(channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
				return err
			}
			err := r.syncPhysicalChannel(ctx, subscription, channel, true)
			if err != nil {
				logging.FromContext(ctx).Warn("Failed to sync physical from Channel", zap.Error(err))
				r.Recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
				return err
			}
		}
		removeFinalizer(subscription)
		_, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(subscription.Namespace).Update(subscription)
		return err
	}
	channel, err := r.getChannelable(ctx, subscription.Namespace, &subscription.Spec.Channel)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get Spec.Channel as Channelable duck type",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
		subscription.Status.MarkReferencesNotResolved(channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
		return err
	}
	if err := r.validateChannel(ctx, channel); err != nil {
		logging.FromContext(ctx).Warn("Failed to validate Channel",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFailed, "Failed to validate spec.channel: %v", err)
		subscription.Status.MarkReferencesNotResolved(channelReferenceFailed, "Failed to validate spec.channel: %v", err)
		return err
	}

	subscriberURI, err := eventingduck.SubscriberSpec(ctx, r.DynamicClientSet, subscription.Namespace, subscription.Spec.Subscriber, track)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve Subscriber",
			zap.Error(err),
			zap.Any("subscriber", subscription.Spec.Subscriber))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		subscription.Status.MarkReferencesNotResolved(subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		return err
	}

	subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
	logging.FromContext(ctx).Debug("Resolved Subscriber", zap.String("subscriberURI", subscriberURI))

	replyURI, err := r.resolveResult(ctx, subscription.Namespace, subscription.Spec.Reply, track)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve reply",
			zap.Error(err),
			zap.Any("reply", subscription.Spec.Reply))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, replyResolveFailed, "Failed to resolve spec.reply: %v", err)
		subscription.Status.MarkReferencesNotResolved(replyResolveFailed, "Failed to resolve spec.reply: %v", err)
		return err
	}

	subscription.Status.PhysicalSubscription.ReplyURI = replyURI
	logging.FromContext(ctx).Debug("Resolved reply", zap.String("replyURI", replyURI))

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	if err := r.ensureFinalizer(subscription); err != nil {
		return err
	}

	// Check if the subscription needs to be added to the channel
	if !r.subPresentInChannelSpec(subscription, channel) {
		// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
		// the Channel with this information.
		if err := r.syncPhysicalChannel(ctx, subscription, channel, false); err != nil {
			logging.FromContext(ctx).Warn("Failed to sync physical Channel", zap.Error(err))
			r.Recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
			subscription.Status.MarkNotAddedToChannel(physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
			return err
		}
	}
	subscription.Status.MarkAddedToChannel()

	// Check if the subscription is marked as ready by channel.
	// Refresh subscribableChan to avoid another reconile loop.
	// If it doesn't get refreshed, then next reconcile loop will get the updated channel
	channel, err = r.getChannelable(ctx, subscription.Namespace, &subscription.Spec.Channel)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to get Spec.Channel as Channelable duck type",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
		subscription.Status.MarkChannelNotReady(channelReferenceFailed, "Failed to get Spec.Channel as Channelable duck type. %s", err)
		return err
	}
	if err := r.subMarkedReadyByChannel(subscription, channel); err != nil {
		logging.FromContext(ctx).Warn("Subscription not marked by Channel as Ready.", zap.Error(err))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, subscriptionNotMarkedReadyByChannel, err.Error())
		subscription.Status.MarkChannelNotReady(subscriptionNotMarkedReadyByChannel, "Subscription not marked by Channel as Ready: %s", err)
		return err
	}

	subscription.Status.MarkChannelReady()
	return nil
}

func (r Reconciler) subMarkedReadyByChannel(subscription *v1alpha1.Subscription, channel *eventingduckv1alpha1.Channelable) error {
	subscribableStatus := channel.Status.GetSubscribableTypeStatus()

	if subscribableStatus == nil {
		return fmt.Errorf("channel.Status.SubscribableStatus is nil")
	}
	for _, sub := range subscribableStatus.Subscribers {
		if sub.UID == subscription.GetUID() &&
			sub.ObservedGeneration == subscription.GetGeneration() {
			if sub.Ready == corev1.ConditionTrue {
				return nil
			}
			return fmt.Errorf(sub.Message)
		}
	}
	return fmt.Errorf("subscription %q not present in channel %q subscriber's list", subscription.Name, channel.Name)
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

// Todo: this needs to be changed to use cache rather than a API call each time
func (r *Reconciler) getChannelable(ctx context.Context, namespace string, chanReference *corev1.ObjectReference) (*eventingduckv1alpha1.Channelable, error) {
	s, err := eventingduck.ObjectReference(ctx, r.DynamicClientSet, namespace, chanReference)
	if err != nil {
		return nil, err
	}
	channel := eventingduckv1alpha1.Channelable{}
	err = duck.FromUnstructured(s, &channel)
	if err != nil {
		return nil, err
	}
	return &channel, nil
}

func (r *Reconciler) validateChannel(ctx context.Context, channel *eventingduckv1alpha1.Channelable) error {
	// Check whether the CRD has the label for channels.
	gvr, _ := meta.UnsafeGuessKindToResource(channel.GroupVersionKind())
	crdName := fmt.Sprintf("%s.%s", gvr.Resource, gvr.Group)
	crd, err := r.customResourceDefinitionLister.Get(crdName)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to retrieve the CRD for the channel",
			zap.Any("channel", channel), zap.String("crd", crdName), zap.Error(err))
		return err
	}

	// TODO uncomment this once we have a cluster-level tracker. As of now, it needs a namespace.
	// Tell tracker to reconcile this Subscription whenever its Channel CRD changes.
	//if err := r.tracker.Track(utils.ObjectRef(crd, customResourceDefinitionGVK), subscription); err != nil {
	//	return fmt.Errorf("error tracking channel crd '%s': %v", crd.Name, err)
	//}

	if val, ok := crd.Labels[channelCrdLabelKey]; !ok {
		return fmt.Errorf("crd %q does not contain mandatory label %q", crdName, channelCrdLabelKey)
	} else if val != channelCrdLabelValue {
		return fmt.Errorf("crd label %s has invalid value %q", channelCrdLabelKey, val)
	}
	return nil
}

func isNilOrEmptyReply(reply *v1alpha1.ReplyStrategy) bool {
	return reply == nil || equality.Semantic.DeepEqual(reply, &v1alpha1.ReplyStrategy{})
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Subscription) (*v1alpha1.Subscription, error) {
	subscription, err := r.subscriptionLister.Subscriptions(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(subscription.Status, desired.Status) {
		return subscription, nil
	}

	becomesReady := desired.Status.IsReady() && !subscription.Status.IsReady()

	// Don't modify the informers copy.
	existing := subscription.DeepCopy()
	existing.Status = desired.Status

	sub, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(sub.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("Subscription %q became ready after %v", subscription.Name, duration)
		r.Recorder.Event(subscription, corev1.EventTypeNormal, subscriptionReadinessChanged, fmt.Sprintf("Subscription %q became ready", subscription.Name))
		if err := r.StatsReporter.ReportReady("Subscription", subscription.Namespace, subscription.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for Subscription, %v", err)
		}
	}

	return sub, err
}

func (r *Reconciler) ensureFinalizer(sub *v1alpha1.Subscription) error {
	finalizers := sets.NewString(sub.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(sub.Finalizers, finalizerName),
			"resourceVersion": sub.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.EventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Patch(sub.Name, types.MergePatchType, patch)
	return err
}

// resolveResult resolves the Spec.Result object.
func (r *Reconciler) resolveResult(ctx context.Context, namespace string, replyStrategy *v1alpha1.ReplyStrategy, track eventingduck.Track) (string, error) {
	if isNilOrEmptyReply(replyStrategy) {
		return "", nil
	}

	// Tell tracker to reconcile this Subscription whenever the Channel changes.
	if err := track(*replyStrategy.Channel); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to spec.reply.channel", zap.Error(err))
		return "", err
	}

	obj, err := eventingduck.ObjectReference(ctx, r.DynamicClientSet, namespace, replyStrategy.Channel)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to fetch ReplyStrategy Channel",
			zap.Error(err),
			zap.Any("replyStrategy", replyStrategy))
		return "", err
	}
	s := duckv1alpha1.AddressableType{}
	err = duck.FromUnstructured(obj, &s)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to deserialize Addressable target", zap.Error(err))
		return "", err
	}
	if s.Status.Address != nil {
		if url := s.Status.Address.GetURL(); url.Host != "" {
			return url.String(), nil
		}
	}
	return "", fmt.Errorf("reply.status does not contain address")
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
		if isDeleted && errors.IsNotFound(patchErr) {
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
				DeprecatedRef: &corev1.ObjectReference{
					APIVersion: sub.APIVersion,
					Kind:       sub.Kind,
					Namespace:  sub.Namespace,
					Name:       sub.Name,
					UID:        sub.UID,
				},
				UID:           sub.UID,
				Generation:    sub.Generation,
				SubscriberURI: sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:      sub.Status.PhysicalSubscription.ReplyURI,
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
	patched, err := resourceClient.Patch(origChannel.GetName(), types.MergePatchType, patch, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patched", patched))
	return nil
}

func removeFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Delete(finalizerName)
	sub.Finalizers = finalizers.List()
}
