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
	"time"

	eventingduckv1alpha1 "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	eventingduck "github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Subscriptions"
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "subscription-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	subscriptionReconciled         = "SubscriptionReconciled"
	subscriptionReadinessChanged   = "SubscriptionReadinessChanged"
	subscriptionUpdateStatusFailed = "SubscriptionUpdateStatusFailed"
	physicalChannelSyncFailed      = "PhysicalChannelSyncFailed"
	channelReferenceFetchFailed    = "ChannelReferenceFetchFailed"
	subscriberResolveFailed        = "SubscriberResolveFailed"
	resultResolveFailed            = "ResultResolveFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	subscriptionLister  listers.SubscriptionLister
	addressableInformer eventingduck.AddressableInformer
	tracker             tracker.Interface
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	opt reconciler.Options,
	subscriptionInformer eventinginformers.SubscriptionInformer,
	channelInformer eventinginformers.ChannelInformer,
	addressableInformer eventingduck.AddressableInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:                reconciler.NewBase(opt, controllerAgentName),
		subscriptionLister:  subscriptionInformer.Lister(),
		addressableInformer: addressableInformer,
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")
	subscriptionInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	// Tracker is used to notify us when the resources Subscription depends on change, so that the
	// Subscription needs to reconcile again.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	channelInformer.Informer().AddEventHandler(reconciler.Handler(
		// Call the tracker's OnChanged method, but we've seen the objects coming through this path
		// missing TypeMeta, so ensure it is properly populated.
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1alpha1.SchemeGroupVersion.WithKind("Channel"),
		),
	))

	return impl
}

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
	if apierrs.IsNotFound(err) {
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
	// See if the subscription has been deleted
	if subscription.DeletionTimestamp != nil {
		// If the subscription is Ready, then we have to remove it
		// from the channel's subscriber list.
		if subscription.Status.IsReady() {
			err := r.syncPhysicalChannel(ctx, subscription, true)
			if err != nil {
				logging.FromContext(ctx).Warn("Failed to sync physical from Channel", zap.Error(err))
				r.Recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
				return err
			}
		}
		removeFinalizer(subscription)
		_, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(subscription.Namespace).Update(subscription)
		return err
	}

	// Verify that `channel` exists.
	if _, err := eventingduck.ObjectReference(ctx, r.DynamicClientSet, subscription.Namespace, &subscription.Spec.Channel); err != nil {
		logging.FromContext(ctx).Warn("Failed to validate Channel exists",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFetchFailed, "Failed to validate spec.channel exists: %v", err)
		return err
	}

	track := r.addressableInformer.TrackInNamespace(r.tracker, subscription)
	if err := track(subscription.Spec.Channel); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to spec.channel", zap.Error(err))
		return err
	}

	subscriberURI, err := eventingduck.SubscriberSpec(ctx, r.DynamicClientSet, subscription.Namespace, subscription.Spec.Subscriber, track)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve Subscriber",
			zap.Error(err),
			zap.Any("subscriber", subscription.Spec.Subscriber))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		return err
	}

	subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
	logging.FromContext(ctx).Debug("Resolved Subscriber", zap.String("subscriberURI", subscriberURI))

	replyURI, err := r.resolveResult(ctx, subscription.Namespace, subscription.Spec.Reply, track)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve reply",
			zap.Error(err),
			zap.Any("reply", subscription.Spec.Reply))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, resultResolveFailed, "Failed to resolve spec.reply: %v", err)
		return err
	}

	subscription.Status.PhysicalSubscription.ReplyURI = replyURI
	logging.FromContext(ctx).Debug("Resolved reply", zap.String("replyURI", replyURI))

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	if err := r.ensureFinalizer(subscription); err != nil {
		return err
	}

	// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
	// the Channel with this information.
	if err := r.syncPhysicalChannel(ctx, subscription, false); err != nil {
		logging.FromContext(ctx).Warn("Failed to sync physical Channel", zap.Error(err))
		r.Recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		return err
	}
	// Everything went well, set the fact that subscriptions have been modified
	subscription.Status.MarkChannelReady()
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

	sub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(desired.Namespace).UpdateStatus(existing)
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

func (c *Reconciler) ensureFinalizer(sub *v1alpha1.Subscription) error {
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

	_, err = c.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Patch(sub.Name, types.MergePatchType, patch)
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
	if s.Status.Address != nil && s.Status.Address.Hostname != "" {
		return eventingduck.DomainToURL(s.Status.Address.Hostname), nil
	}
	return "", fmt.Errorf("reply.status does not contain address")
}

func (r *Reconciler) syncPhysicalChannel(ctx context.Context, sub *v1alpha1.Subscription, isDeleted bool) error {
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

	if patchErr := r.patchPhysicalFrom(ctx, sub.Namespace, sub.Spec.Channel, subscribable); patchErr != nil {
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
	for _, sub := range subs {
		if sub.Status.PhysicalSubscription.SubscriberURI != "" || sub.Status.PhysicalSubscription.ReplyURI != "" {
			rv.Subscribers = append(rv.Subscribers, eventingduckv1alpha1.ChannelSubscriberSpec{
				DeprecatedRef: &corev1.ObjectReference{
					APIVersion: sub.APIVersion,
					Kind:       sub.Kind,
					Namespace:  sub.Namespace,
					Name:       sub.Name,
					UID:        sub.UID,
				},
				UID:           sub.UID,
				SubscriberURI: sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:      sub.Status.PhysicalSubscription.ReplyURI,
			})
		}
	}
	return rv
}

func (r *Reconciler) patchPhysicalFrom(ctx context.Context, namespace string, physicalFrom corev1.ObjectReference, subs *eventingduckv1alpha1.Subscribable) error {
	// First get the original object and convert it to only the bits we care about
	s, err := eventingduck.ObjectReference(ctx, r.DynamicClientSet, namespace, &physicalFrom)
	if err != nil {
		return err
	}
	original := eventingduckv1alpha1.Channel{}
	err = duck.FromUnstructured(s, &original)
	if err != nil {
		return err
	}

	after := original.DeepCopy()
	after.Spec.Subscribable = subs

	patch, err := duck.CreateMergePatch(original, after)
	if err != nil {
		return err
	}

	resourceClient, err := eventingduck.ResourceInterface(r.DynamicClientSet, namespace, &physicalFrom)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return err
	}
	patched, err := resourceClient.Patch(original.Name, types.MergePatchType, patch, metav1.UpdateOptions{})
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
