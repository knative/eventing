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

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/utils/resolve"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "subscription-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	subscriptionReconciled         = "SubscriptionReconciled"
	subscriptionUpdateStatusFailed = "SubscriptionUpdateStatusFailed"
	physicalChannelSyncFailed      = "PhysicalChannelSyncFailed"
	channelReferenceFetchFailed    = "ChannelReferenceFetchFailed"
	subscriberResolveFailed        = "SubscriberResolveFailed"
	resultResolveFailed            = "ResultResolveFailed"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
	logger        *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a Subscription controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			logger:   logger,
		},
	})
	if err != nil {
		return nil, err
	}

	// Watch Subscription events and enqueue Subscription object key.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Subscription resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := logging.WithLogger(context.TODO(), r.logger.With(zap.Any("request", request)))
	logging.FromContext(ctx).Debug("Reconciling Subscription")
	subscription := &v1alpha1.Subscription{}
	err := r.client.Get(ctx, request.NamespacedName, subscription)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Could not find Subscription")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Error getting Subscription", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Reconcile this copy of the Subscription and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, subscription)
	if err != nil {
		logging.FromContext(ctx).Warn("Error reconciling Subscription", zap.Error(err))
	} else {
		logging.FromContext(ctx).Debug("Subscription reconciled")
		r.recorder.Eventf(subscription, corev1.EventTypeNormal, subscriptionReconciled, "Subscription reconciled: %q", subscription.Name)
	}

	if _, updateStatusErr := r.updateStatus(ctx, subscription.DeepCopy()); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Failed to update the Subscription", zap.Error(err))
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, subscriptionUpdateStatusFailed, "Failed to update Subscription's status: %v", err)
		return reconcile.Result{}, updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, subscription *v1alpha1.Subscription) error {
	subscription.Status.InitializeConditions()

	// See if the subscription has been deleted
	if subscription.DeletionTimestamp != nil {
		// If the subscription is Ready, then we have to remove it
		// from the channel's subscriber list.
		if subscription.Status.IsReady() {
			err := r.syncPhysicalChannel(ctx, subscription, true)
			if err != nil {
				logging.FromContext(ctx).Warn("Failed to sync physical from Channel", zap.Error(err))
				r.recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
				return err
			}
		}
		removeFinalizer(subscription)
		return nil
	}

	// Verify that `channel` exists.
	if _, err := resolve.ObjectReference(ctx, r.dynamicClient, subscription.Namespace, &subscription.Spec.Channel); err != nil {
		logging.FromContext(ctx).Warn("Failed to validate Channel exists",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFetchFailed, "Failed to validate spec.channel exists: %v", err)
		return err
	}

	if subscriberURI, err := resolve.SubscriberSpec(ctx, r.dynamicClient, subscription.Namespace, subscription.Spec.Subscriber); err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve Subscriber",
			zap.Error(err),
			zap.Any("subscriber", subscription.Spec.Subscriber))
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		return err
	} else {
		subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
		logging.FromContext(ctx).Debug("Resolved Subscriber", zap.String("subscriberURI", subscriberURI))
	}

	if replyURI, err := r.resolveResult(ctx, subscription.Namespace, subscription.Spec.Reply); err != nil {
		logging.FromContext(ctx).Warn("Failed to resolve reply",
			zap.Error(err),
			zap.Any("reply", subscription.Spec.Reply))
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, resultResolveFailed, "Failed to resolve spec.reply: %v", err)
		return err
	} else {
		subscription.Status.PhysicalSubscription.ReplyURI = replyURI
		logging.FromContext(ctx).Debug("Resolved reply", zap.String("replyURI", replyURI))
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
	// the Channel with this information.
	if err := r.syncPhysicalChannel(ctx, subscription, false); err != nil {
		logging.FromContext(ctx).Warn("Failed to sync physical Channel", zap.Error(err))
		r.recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		return err
	}
	// Everything went well, set the fact that subscriptions have been modified
	subscription.Status.MarkChannelReady()
	addFinalizer(subscription)
	return nil
}

func isNilOrEmptyReply(reply *v1alpha1.ReplyStrategy) bool {
	return reply == nil || equality.Semantic.DeepEqual(reply, &v1alpha1.ReplyStrategy{})
}

// updateStatus may in fact update the subscription's finalizers in addition to the status
func (r *reconciler) updateStatus(ctx context.Context, subscription *v1alpha1.Subscription) (*v1alpha1.Subscription, error) {
	objectKey := client.ObjectKey{Namespace: subscription.Namespace, Name: subscription.Name}
	latestSubscription := &v1alpha1.Subscription{}

	if err := r.client.Get(ctx, objectKey, latestSubscription); err != nil {
		return nil, err
	}

	subscriptionChanged := false

	if !equality.Semantic.DeepEqual(latestSubscription.Finalizers, subscription.Finalizers) {
		latestSubscription.SetFinalizers(subscription.ObjectMeta.Finalizers)
		if err := r.client.Update(ctx, latestSubscription); err != nil {
			return nil, err
		}
		subscriptionChanged = true
	}

	if equality.Semantic.DeepEqual(latestSubscription.Status, subscription.Status) {
		return latestSubscription, nil
	}

	if subscriptionChanged {
		// Refetch
		latestSubscription = &v1alpha1.Subscription{}
		if err := r.client.Get(ctx, objectKey, latestSubscription); err != nil {
			return nil, err
		}
	}

	latestSubscription.Status = subscription.Status
	if err := r.client.Status().Update(ctx, latestSubscription); err != nil {
		return nil, err
	}

	return latestSubscription, nil
}

// resolveResult resolves the Spec.Result object.
func (r *reconciler) resolveResult(ctx context.Context, namespace string, replyStrategy *v1alpha1.ReplyStrategy) (string, error) {
	if isNilOrEmptyReply(replyStrategy) {
		return "", nil
	}
	obj, err := resolve.ObjectReference(ctx, r.dynamicClient, namespace, replyStrategy.Channel)
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
		return resolve.DomainToURL(s.Status.Address.Hostname), nil
	}
	return "", fmt.Errorf("status does not contain address")
}

func (r *reconciler) syncPhysicalChannel(ctx context.Context, sub *v1alpha1.Subscription, isDeleted bool) error {
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

func (r *reconciler) listAllSubscriptionsWithPhysicalChannel(ctx context.Context, sub *v1alpha1.Subscription) ([]v1alpha1.Subscription, error) {
	subs := make([]v1alpha1.Subscription, 0)

	opts := &client.ListOptions{
		Namespace: sub.Namespace,
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		sl := &v1alpha1.SubscriptionList{}
		err := r.client.List(ctx, opts, sl)
		if err != nil {
			return nil, err
		}
		for _, s := range sl.Items {
			if sub.UID == s.UID {
				// This is the sub that is being reconciled. Skip it.
				continue
			}
			if equality.Semantic.DeepEqual(sub.Spec.Channel, s.Spec.Channel) {
				subs = append(subs, s)
			}
		}
		if sl.Continue != "" {
			opts.Raw.Continue = sl.Continue
		} else {
			return subs, nil
		}
	}
}

func (r *reconciler) createSubscribable(subs []v1alpha1.Subscription) *eventingduck.Subscribable {
	rv := &eventingduck.Subscribable{}
	for _, sub := range subs {
		if sub.Status.PhysicalSubscription.SubscriberURI != "" || sub.Status.PhysicalSubscription.ReplyURI != "" {
			rv.Subscribers = append(rv.Subscribers, eventingduck.ChannelSubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: sub.APIVersion,
					Kind:       sub.Kind,
					Namespace:  sub.Namespace,
					Name:       sub.Name,
					UID:        sub.UID,
				},
				SubscriberURI: sub.Status.PhysicalSubscription.SubscriberURI,
				ReplyURI:      sub.Status.PhysicalSubscription.ReplyURI,
			})
		}
	}
	return rv
}

func (r *reconciler) patchPhysicalFrom(ctx context.Context, namespace string, physicalFrom corev1.ObjectReference, subs *eventingduck.Subscribable) error {
	// First get the original object and convert it to only the bits we care about
	s, err := resolve.ObjectReference(ctx, r.dynamicClient, namespace, &physicalFrom)
	if err != nil {
		return err
	}
	original := eventingduck.Channel{}
	err = duck.FromUnstructured(s, &original)
	if err != nil {
		return err
	}

	after := original.DeepCopy()
	after.Spec.Subscribable = subs

	patch, err := duck.CreatePatch(original, after)
	if err != nil {
		return err
	}

	patchBytes, err := patch.MarshalJSON()
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to marshal JSON patch", zap.Error(err), zap.Any("patch", patch))
		return err
	}

	resourceClient, err := resolve.ResourceInterface(r.dynamicClient, namespace, &physicalFrom)
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to create dynamic resource client", zap.Error(err))
		return err
	}
	patched, err := resourceClient.Patch(original.Name, types.JSONPatchType, patchBytes, metav1.UpdateOptions{})
	if err != nil {
		logging.FromContext(ctx).Warn("Failed to patch the Channel", zap.Error(err), zap.Any("patch", patch))
		return err
	}
	logging.FromContext(ctx).Debug("Patched resource", zap.Any("patched", patched))
	return nil
}

func addFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Insert(finalizerName)
	sub.Finalizers = finalizers.List()
}

func removeFinalizer(sub *v1alpha1.Subscription) {
	finalizers := sets.NewString(sub.Finalizers...)
	finalizers.Delete(finalizerName)
	sub.Finalizers = finalizers.List()
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}
