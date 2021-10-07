/*
Copyright 2019 The Knative Authors

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

package parallel

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"

	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	duckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	parallelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/parallel"
	listers "knative.dev/eventing/pkg/client/listers/flows/v1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	ducklib "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
)

type Reconciler struct {
	// listers index properties about resources
	parallelLister     listers.ParallelLister
	channelableTracker ducklib.ListableTracker
	subscriptionLister messaginglisters.SubscriptionLister

	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface

	// dynamicClientSet allows us to configure pluggable Build objects
	dynamicClientSet dynamic.Interface
}

// Check that our Reconciler implements parallelreconciler.Interface
var _ parallelreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, p *v1.Parallel) pkgreconciler.Event {
	// Reconciling parallel is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole parallel and one filter channel per branch.
	// 2. For each of the Branches:
	//     2.1 create a Subscription to the fronting Channel, subscribe the filter and send reply to the filter Channel
	//     2.2 create a Subscription to the filter Channel, subscribe the subscriber and send reply to
	//         either the branch Reply. If not present, send reply to the global Reply. If not present, do not send reply.
	// 3. Rinse and repeat step #2 above for each branch in the list

	gvr, _ := meta.UnsafeGuessKindToResource(p.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.dynamicClientSet.Resource(gvr).Namespace(p.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", p.Spec.ChannelTemplate)
	}

	var ingressChannel *duckv1.Channelable
	channels := make([]*duckv1.Channelable, 0, len(p.Spec.Branches))
	for i := -1; i < len(p.Spec.Branches); i++ {
		var channelName string
		if i == -1 {
			channelName = resources.ParallelChannelName(p.Name)
		} else {
			channelName = resources.ParallelBranchChannelName(p.Name, i)
		}

		channelObjRef := corev1.ObjectReference{
			Kind:       p.Spec.ChannelTemplate.Kind,
			APIVersion: p.Spec.ChannelTemplate.APIVersion,
			Name:       channelName,
			Namespace:  p.Namespace,
		}

		channelable, err := r.reconcileChannel(ctx, channelResourceInterface, p, channelObjRef)
		if err != nil {
			logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", p.Namespace, channelName), zap.Error(err))
			return err
		}
		logging.FromContext(ctx).Infof("Reconciled Channel Object: %s/%s %+v", p.Namespace, channelName, channelable)

		if i == -1 {
			ingressChannel = channelable
		} else {
			channels = append(channels, channelable)
		}
	}
	p.Status.PropagateChannelStatuses(ingressChannel, channels)

	filterSubs := make([]*messagingv1.Subscription, 0, len(p.Spec.Branches))
	subs := make([]*messagingv1.Subscription, 0, len(p.Spec.Branches))
	for i := 0; i < len(p.Spec.Branches); i++ {
		filterSub, sub, err := r.reconcileBranch(ctx, i, p)
		if err != nil {
			return fmt.Errorf("failed to reconcile Subscription Objects for branch: %d : %s", i, err)
		}
		subs = append(subs, sub)
		filterSubs = append(filterSubs, filterSub)
		logging.FromContext(ctx).Debugf("Reconciled Subscription Objects for branch: %d: %+v, %+v", i, filterSub, sub)
	}
	p.Status.PropagateSubscriptionStatuses(filterSubs, subs)

	// If a parallel instance is modified resulting in the number of steps decreasing, there will be
	// leftover channels and subscriptions that need to be removed.
	if err := r.removeUnwantedChannels(ctx, channelResourceInterface, p, append(channels, ingressChannel)); err != nil {
		return fmt.Errorf("error removing unwanted Channels: %w", err)
	}

	if err := r.removeUnwantedSubscriptions(ctx, p, append(filterSubs, subs...)); err != nil {
		return fmt.Errorf("error removing unwanted Subscriptions: %w", err)
	}

	return nil
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, p *v1.Parallel, channelObjRef corev1.ObjectReference) (*duckv1.Channelable, error) {
	logger := logging.FromContext(ctx)
	c, err := r.trackAndFetchChannel(ctx, p, channelObjRef)
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := ducklib.NewPhysicalChannel(
				p.Spec.ChannelTemplate.TypeMeta,
				metav1.ObjectMeta{
					Name:      channelObjRef.Name,
					Namespace: p.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(p),
					},
				},
				ducklib.WithPhysicalChannelSpec(p.Spec.ChannelTemplate.Spec),
			)
			logger.Errorf("Creating Channel Object: %+v", newChannel)
			if err != nil {
				logger.Errorw("Failed to create Channel resource object", zap.Any("channel", channelObjRef), zap.Error(err))
				return nil, err
			}
			created, err := channelResourceInterface.Create(ctx, newChannel, metav1.CreateOptions{})
			if err != nil {
				logger.Errorw("Failed to create Channel", zap.Any("channel", channelObjRef), zap.Error(err))
				return nil, err
			}
			logger.Debugw("Created Channel", zap.Any("channel", newChannel))
			// Convert to Channel duck so that we can treat all Channels the same.
			channelable := &duckv1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logger.Errorw("Failed to convert to Channelable Object", zap.Any("channel", created), zap.Error(err))
				return nil, err
			}
			return channelable, nil
		}
		logger.Errorw("Failed to get Channel", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	logger.Debugw("Found Channel", zap.Any("channel", channelObjRef))
	channelable, ok := c.(*duckv1.Channelable)
	if !ok {
		logger.Errorw("Failed to convert to Channelable Object", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, fmt.Errorf("failed to convert to Channelable Object: %+v", c)
	}
	return channelable, nil
}

func (r *Reconciler) reconcileBranch(ctx context.Context, branchNumber int, p *v1.Parallel) (*messagingv1.Subscription, *messagingv1.Subscription, error) {
	filterExpected := resources.NewFilterSubscription(branchNumber, p)
	filterSub, err := r.reconcileSubscription(ctx, branchNumber, filterExpected)
	if err != nil {
		return nil, nil, err
	}

	expected := resources.NewSubscription(branchNumber, p)
	sub, err := r.reconcileSubscription(ctx, branchNumber, expected)
	if err != nil {
		return nil, nil, err
	}

	return filterSub, sub, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, branchNumber int, expected *messagingv1.Subscription) (*messagingv1.Subscription, error) {
	sub, err := r.subscriptionLister.Subscriptions(expected.Namespace).Get(expected.Name)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Infof("Creating subscription: %+v", sub)
		newSub, err := r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Create(ctx, sub, metav1.CreateOptions{})
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Parallel's subscription failed: %v", err)
			return nil, fmt.Errorf("failed to create Subscription Object for branch: %d : %s", branchNumber, err)
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Errorw("Failed to get Subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Parallels's subscription failed: %v", err)
		return nil, fmt.Errorf("failed to get Subscription: %s", err)
	} else if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Delete(ctx, sub.Name, metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Infow("Cannot delete Subscription", zap.Error(err))
			return nil, err
		}
		newSub, err := r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			logging.FromContext(ctx).Infow("Cannot create Subscription", zap.Error(err))
			return nil, err
		}
		return newSub, nil
	}
	return sub, nil
}

func (r *Reconciler) trackAndFetchChannel(ctx context.Context, p *v1.Parallel, ref corev1.ObjectReference) (runtime.Object, pkgreconciler.Event) {
	// Track the channel using the channelableTracker.
	// We don't need the explicitly set a channelInformer, as this will dynamically generate one for us.
	// This code needs to be called before checking the existence of the `channel`, in order to make sure the
	// subscription controller will reconcile upon a `channel` change.
	if err := r.channelableTracker.TrackInNamespace(ctx, p)(ref); err != nil {
		return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, "TrackerFailed", "unable to track changes to Channel %+v : %w", ref, err)
	}
	chLister, err := r.channelableTracker.ListerFor(ref)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error getting lister for Channel", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	obj, err := chLister.ByNamespace(p.Namespace).Get(ref.Name)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error getting Channel from lister", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	return obj, err
}

func (r *Reconciler) removeUnwantedChannels(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, p *v1.Parallel, wanted []*duckv1.Channelable) error {
	channelObjRef := corev1.ObjectReference{
		Kind:       p.Spec.ChannelTemplate.Kind,
		APIVersion: p.Spec.ChannelTemplate.APIVersion,
	}

	l, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		return fmt.Errorf("error getting lister for Channels: %w", err)
	}

	ownedChannels, err := l.ByNamespace(p.GetNamespace()).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing Channels: %w", err)
	}

	ownedSet := sets.String{}
	for _, c := range ownedChannels {
		ch, err := kmeta.DeletionHandlingAccessor(c)
		if err != nil {
			return fmt.Errorf("error reading Channel %q: %w", ch.GetName(), err)
		}

		if !ch.GetDeletionTimestamp().IsZero() ||
			!metav1.IsControlledBy(ch, p) {
			continue
		}

		ownedSet.Insert(ch.GetName())
	}

	wantedSet := sets.String{}
	for _, cw := range wanted {
		wantedSet.Insert(cw.Name)
	}

	for _, c := range ownedSet.Difference(wantedSet).List() {
		err = channelResourceInterface.Delete(ctx, c, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting Channel %q: %w", c, err)
		}
	}

	return nil
}

func (r *Reconciler) removeUnwantedSubscriptions(ctx context.Context, p *v1.Parallel, wanted []*messagingv1.Subscription) error {
	subs, err := r.subscriptionLister.Subscriptions(p.Namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing Subscriptions: %w", err)
	}

	ownedSet := sets.String{}
	for _, sub := range subs {
		if !sub.GetDeletionTimestamp().IsZero() ||
			!metav1.IsControlledBy(sub, p) {
			continue
		}

		ownedSet.Insert(sub.GetName())
	}

	wantedSet := sets.String{}
	for _, sw := range wanted {
		wantedSet.Insert(sw.Name)
	}

	for _, s := range ownedSet.Difference(wantedSet).List() {
		err = r.eventingClientSet.MessagingV1().Subscriptions(p.Namespace).Delete(ctx, s, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("error deleting Subscription %q: %w", s, err)

		}
	}

	return nil
}
