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

	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	eventingduck "github.com/knative/eventing/pkg/apis/duck/v1alpha1"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/utils/resolve"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	physicalChannelSyncFailed   = "PhysicalChannelSyncFailed"
	channelReferenceFetchFailed = "ChannelReferenceFetchFailed"
	subscriberResolveFailed     = "SubscriberResolveFailed"
	resultResolveFailed         = "ResultResolveFailed"
)

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
}

// Verify the struct implements necessary interfaces
var _ eventingreconciler.EventingReconciler = &reconciler{}
var _ eventingreconciler.Finalizer = &reconciler{}
var _ inject.Config = &reconciler{}

// ProvideController returns a Subscription controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	logger = logger.With(zap.String("controller", controllerAgentName))

	r, err := eventingreconciler.New(
		&reconciler{},
		logger,
		mgr.GetRecorder(controllerAgentName),
		eventingreconciler.EnableFinalizer(finalizerName),
		eventingreconciler.EnableConfigInjection(),
	)
	if err != nil {
		return nil, err
	}

	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch Subscription events and enqueue Subscription object key.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Subscription{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	return c, nil
}

func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	// Do not want to handle this error. It is better to panic here because this points to erroneous GetNewReconcileObject() function
	subscription := obj.(*v1alpha1.Subscription)
	subscription.Status.InitializeConditions()
	logger := logging.FromContext(ctx)

	// Verify that `channel` exists.
	if _, err := resolve.ObjectReference(ctx, r.dynamicClient, subscription.Namespace, &subscription.Spec.Channel); err != nil {
		logger.Warn("Failed to validate Channel exists",
			zap.Error(err),
			zap.Any("channel", subscription.Spec.Channel))
		recorder.Eventf(subscription, corev1.EventTypeWarning, channelReferenceFetchFailed, "Failed to validate spec.channel exists: %v", err)
		return true, reconcile.Result{}, err
	}

	if subscriberURI, err := resolve.SubscriberSpec(ctx, r.dynamicClient, subscription.Namespace, subscription.Spec.Subscriber); err != nil {
		logger.Warn("Failed to resolve Subscriber",
			zap.Error(err),
			zap.Any("subscriber", subscription.Spec.Subscriber))
		recorder.Eventf(subscription, corev1.EventTypeWarning, subscriberResolveFailed, "Failed to resolve spec.subscriber: %v", err)
		return true, reconcile.Result{}, err
	} else {
		subscription.Status.PhysicalSubscription.SubscriberURI = subscriberURI
		logger.Debug("Resolved Subscriber", zap.String("subscriberURI", subscriberURI))
	}

	if replyURI, err := r.resolveResult(ctx, subscription.Namespace, subscription.Spec.Reply); err != nil {
		logger.Warn("Failed to resolve reply",
			zap.Error(err),
			zap.Any("reply", subscription.Spec.Reply))
		recorder.Eventf(subscription, corev1.EventTypeWarning, resultResolveFailed, "Failed to resolve spec.reply: %v", err)
		return true, reconcile.Result{}, err
	} else {
		subscription.Status.PhysicalSubscription.ReplyURI = replyURI
		logger.Debug("Resolved reply", zap.String("replyURI", replyURI))
	}

	// Everything that was supposed to be resolved was, so flip the status bit on that.
	subscription.Status.MarkReferencesResolved()

	// Ok, now that we have the Channel and at least one of the Call/Result, let's reconcile
	// the Channel with this information.
	if err := r.syncPhysicalChannel(ctx, subscription, false); err != nil {
		logger.Warn("Failed to sync physical Channel", zap.Error(err))
		recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
		return true, reconcile.Result{}, err
	}
	// Everything went well, set the fact that subscriptions have been modified
	subscription.Status.MarkChannelReady()
	return true, reconcile.Result{}, nil
}

func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &v1alpha1.Subscription{}
}

func (r *reconciler) OnDelete(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) error {
	// Do not want to handle this error. It is better to panic here because this points to erroneous GetNewReconcileObject() function
	subscription := obj.(*v1alpha1.Subscription)
	if subscription.Status.IsReady() {
		err := r.syncPhysicalChannel(ctx, subscription, true)
		if err != nil {
			logging.FromContext(ctx).Warn("Failed to sync physical from Channel", zap.Error(err))
			recorder.Eventf(subscription, corev1.EventTypeWarning, physicalChannelSyncFailed, "Failed to sync physical Channel: %v", err)
			return err
		}
	}
	return nil
}

func isNilOrEmptySubscriber(sub *v1alpha1.SubscriberSpec) bool {
	return sub == nil || equality.Semantic.DeepEqual(sub, &v1alpha1.SubscriberSpec{})
}

func isNilOrEmptyReply(reply *v1alpha1.ReplyStrategy) bool {
	return reply == nil || equality.Semantic.DeepEqual(reply, &v1alpha1.ReplyStrategy{})
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
		if isDeleted && apierrors.IsNotFound(patchErr) {
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

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
