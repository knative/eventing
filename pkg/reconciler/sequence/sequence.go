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

package sequence

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

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/flows/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	sequencereconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1alpha1/sequence"
	listers "knative.dev/eventing/pkg/client/listers/flows/v1alpha1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	pkgreconciler "knative.dev/pkg/reconciler"
)

func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SequenceReconciled", "Sequence reconciled: \"%s/%s\"", namespace, name)
}

type Reconciler struct {
	// listers index properties about resources
	sequenceLister     listers.SequenceLister
	channelableTracker duck.ListableTracker
	subscriptionLister messaginglisters.SubscriptionLister

	// eventingClientSet allows us to configure Eventing objects
	eventingClientSet clientset.Interface

	// dynamicClientSet allows us to configure pluggable Build objects
	dynamicClientSet dynamic.Interface
}

// Check that our Reconciler implements sequencereconciler.Interface
var _ sequencereconciler.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, s *v1alpha1.Sequence) pkgreconciler.Event {
	s.Status.InitializeConditions()
	s.Status.ObservedGeneration = s.Generation

	// Reconciling sequence is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole sequence
	// 2. For each of the Steps, create a Subscription to the previous Channel
	//    (hence the first step above for the first step in the "steps"), where the Subscriber points to the
	//    Step, and create intermediate channel for feeding the Reply to (if we allow Reply to be something else
	//    than channel, we could just (optionally) feed it directly to the following subscription.
	// 3. Rinse and repeat step #2 above for each Step in the list
	// 4. If there's a Reply, then the last Subscription will be configured to send the reply to that.

	gvr, _ := meta.UnsafeGuessKindToResource(s.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())
	channelResourceInterface := r.dynamicClientSet.Resource(gvr).Namespace(s.Namespace)
	if channelResourceInterface == nil {
		return fmt.Errorf("unable to create dynamic client for: %+v", s.Spec.ChannelTemplate)
	}

	// Tell tracker to reconcile this Sequence whenever my channels change.
	track := r.channelableTracker.TrackInNamespace(s)

	channels := make([]*duckv1alpha1.Channelable, 0, len(s.Spec.Steps))
	for i := 0; i < len(s.Spec.Steps); i++ {
		ingressChannelName := resources.SequenceChannelName(s.Name, i)

		channelObjRef := corev1.ObjectReference{
			Kind:       s.Spec.ChannelTemplate.Kind,
			APIVersion: s.Spec.ChannelTemplate.APIVersion,
			Name:       ingressChannelName,
			Namespace:  s.Namespace,
		}
		// Track channels and enqueue sequence when they change.
		if err := track(channelObjRef); err != nil {
			return fmt.Errorf("unable to track changes to Channel: %v", err)
		}

		channelable, err := r.reconcileChannel(ctx, channelResourceInterface, s, channelObjRef)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", s.Namespace, ingressChannelName), zap.Error(err))
			return err

		}
		channels = append(channels, channelable)
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Channel Object: %s/%s %+v", s.Namespace, ingressChannelName, channelable))
	}
	s.Status.PropagateChannelStatuses(channels)

	subs := make([]*messagingv1alpha1.Subscription, 0, len(s.Spec.Steps))
	for i := 0; i < len(s.Spec.Steps); i++ {
		sub, err := r.reconcileSubscription(ctx, i, s)
		if err != nil {
			return fmt.Errorf("failed to reconcile Subscription Object for step: %d : %s", i, err)
		}
		subs = append(subs, sub)
		logging.FromContext(ctx).Debug(fmt.Sprintf("Reconciled Subscription Object for step: %d: %+v", i, sub))
	}
	s.Status.PropagateSubscriptionStatuses(subs)

	return newReconciledNormal(s.Namespace, s.Name)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, s *v1alpha1.Sequence, channelObjRef corev1.ObjectReference) (*duckv1alpha1.Channelable, error) {
	lister, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Error("Error getting lister for Channel", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	c, err := lister.ByNamespace(channelObjRef.Namespace).Get(channelObjRef.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := resources.NewChannel(channelObjRef.Name, s)
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

func (r *Reconciler) reconcileSubscription(ctx context.Context, step int, p *v1alpha1.Sequence) (*messagingv1alpha1.Subscription, error) {
	expected := resources.NewSubscription(step, p)

	subName := resources.SequenceSubscriptionName(p.Name, step)
	sub, err := r.subscriptionLister.Subscriptions(p.Namespace).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating subscription: %+v", sub))
		newSub, err := r.eventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequence's subscription failed: %v", err)
			return nil, fmt.Errorf("failed to create Subscription Object for step: %d : %s", step, err)
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequences's subscription failed: %v", err)
		return nil, fmt.Errorf("failed to get subscription: %s", err)
	} else if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.eventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			return nil, err
		}
		newSub, err := r.eventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Create(expected)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			return nil, err
		}
		return newSub, nil
	}
	return sub, nil
}
