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
	"k8s.io/client-go/dynamic"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/tracker"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"

	parallelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1alpha1/parallel"
	listers "knative.dev/eventing/pkg/client/listers/flows/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/parallel/resources"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason ParallelReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "ParallelReconciled", "Parallel reconciled: \"%s/%s\"", namespace, name)
}

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	parallelLister     listers.ParallelLister
	tracker            tracker.Interface
	channelableTracker duck.ListableTracker
	subscriptionLister messaginglisters.SubscriptionLister
}

// Check that our Reconciler implements parallelreconciler.Interface
var _ parallelreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, p *v1alpha1.Parallel) pkgreconciler.Event {
	p.Status.InitializeConditions()
	p.Status.ObservedGeneration = p.Generation

	// Reconciling parallel is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole parallel and one filter channel per branch.
	// 2. For each of the Branches:
	//     2.1 create a Subscription to the fronting Channel, subscribe the filter and send reply to the filter Channel
	//     2.2 create a Subscription to the filter Channel, subcribe the subscriber and send reply to
	//         either the branch Reply. If not present, send reply to the global Reply. If not present, do not send reply.
	// 3. Rinse and repeat step #2 above for each branch in the list
	if p.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	gvr, _ := meta.UnsafeGuessKindToResource(p.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.DynamicClientSet.Resource(gvr).Namespace(p.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", p.Spec.ChannelTemplate)
	}

	// Tell tracker to reconcile this Parallel whenever my channels change.
	track := r.channelableTracker.TrackInNamespace(p)

	var ingressChannel *duckv1alpha1.Channelable
	channels := make([]*duckv1alpha1.Channelable, 0, len(p.Spec.Branches))
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

		// Track channels and enqueue parallel when they change.
		if err := track(channelObjRef); err != nil {
			return fmt.Errorf("unable to track changes to Channel: %v", err)
		}

		channelable, err := r.reconcileChannel(ctx, channelResourceInterface, p, channelObjRef)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", p.Namespace, channelName), zap.Error(err))
			return err
		}
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Channel Object: %s/%s %+v", p.Namespace, channelName, channelable))

		if i == -1 {
			ingressChannel = channelable
		} else {
			channels = append(channels, channelable)
		}
	}
	p.Status.PropagateChannelStatuses(ingressChannel, channels)

	filterSubs := make([]*messagingv1alpha1.Subscription, 0, len(p.Spec.Branches))
	subs := make([]*messagingv1alpha1.Subscription, 0, len(p.Spec.Branches))
	for i := 0; i < len(p.Spec.Branches); i++ {
		filterSub, sub, err := r.reconcileBranch(ctx, i, p)
		if err != nil {
			return fmt.Errorf("failed to reconcile Subscription Objects for branch: %d : %s", i, err)
		}
		subs = append(subs, sub)
		filterSubs = append(filterSubs, filterSub)
		logging.FromContext(ctx).Debug(fmt.Sprintf("Reconciled Subscription Objects for branch: %d: %+v, %+v", i, filterSub, sub))
	}
	p.Status.PropagateSubscriptionStatuses(filterSubs, subs)

	return newReconciledNormal(p.Namespace, p.Name)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, p *v1alpha1.Parallel, channelObjRef corev1.ObjectReference) (*duckv1alpha1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Error("Error getting lister for Channel", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	c, err := lister.ByNamespace(channelObjRef.Namespace).Get(channelObjRef.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := resources.NewChannel(channelObjRef.Name, p)
			logging.FromContext(ctx).Error(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create Channel resource object", zap.Any("channel", channelObjRef), zap.Error(err))
				return nil, err
			}
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("Failed to create Channel", zap.Any("channel", channelObjRef), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Debug("Created Channel", zap.Any("channel", newChannel))
			// Convert to Channel duck so that we can treat all Channels the same.
			channelable := &duckv1alpha1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("channel", channelObjRef), zap.Error(err))
				return nil, err
			}
			return channelable, nil
		}
		logging.FromContext(ctx).Error("Failed to get Channel", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	logging.FromContext(ctx).Debug("Found Channel", zap.Any("channel", channelObjRef))
	channelable, ok := c.(*duckv1alpha1.Channelable)
	if !ok {
		logging.FromContext(ctx).Error("Failed to convert to Channelable Object", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	return channelable, nil
}

func (r *Reconciler) reconcileBranch(ctx context.Context, branchNumber int, p *v1alpha1.Parallel) (*messagingv1alpha1.Subscription, *messagingv1alpha1.Subscription, error) {
	filterExpected := resources.NewFilterSubscription(branchNumber, p)
	filterSubName := resources.ParallelFilterSubscriptionName(p.Name, branchNumber)

	filterSub, err := r.reconcileSubscription(ctx, branchNumber, filterExpected, filterSubName, p.Namespace)
	if err != nil {
		return nil, nil, err
	}

	expected := resources.NewSubscription(branchNumber, p)
	subName := resources.ParallelSubscriptionName(p.Name, branchNumber)

	sub, err := r.reconcileSubscription(ctx, branchNumber, expected, subName, p.Namespace)
	if err != nil {
		return nil, nil, err
	}

	return filterSub, sub, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, branchNumber int, expected *messagingv1alpha1.Subscription, subName, ns string) (*messagingv1alpha1.Subscription, error) {
	sub, err := r.subscriptionLister.Subscriptions(ns).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating subscription: %+v", sub))
		newSub, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Parallel's subscription failed: %v", err)
			return nil, fmt.Errorf("failed to create Subscription Object for branch: %d : %s", branchNumber, err)
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Parallels's subscription failed: %v", err)
		return nil, fmt.Errorf("failed to get subscription: %s", err)
	} else if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.EventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			return nil, err
		}
		newSub, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Create(expected)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			return nil, err
		}
		return newSub, nil
	}
	return sub, nil
}
