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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/kmeta"

	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	sequencereconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/sequence"
	listers "knative.dev/eventing/pkg/client/listers/flows/v1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/eventing/pkg/duck"
	ducklib "knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
)

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
func (r *Reconciler) ReconcileKind(ctx context.Context, s *v1.Sequence) pkgreconciler.Event {
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

	channels := make([]*eventingduckv1.Channelable, 0, len(s.Spec.Steps))
	for i := 0; i < len(s.Spec.Steps); i++ {
		ingressChannelName := resources.SequenceChannelName(s.Name, i)

		channelObjRef := corev1.ObjectReference{
			Kind:       s.Spec.ChannelTemplate.Kind,
			APIVersion: s.Spec.ChannelTemplate.APIVersion,
			Name:       ingressChannelName,
			Namespace:  s.Namespace,
		}

		channelable, err := r.reconcileChannel(ctx, channelResourceInterface, s, channelObjRef)
		if err != nil {
			logging.FromContext(ctx).Errorw(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", s.Namespace, ingressChannelName), zap.Error(err))
			s.Status.MarkChannelsNotReady("ChannelsNotReady", "Failed to reconcile channels, step: %d", i)
			return fmt.Errorf("failed to reconcile channel resource for step: %d : %s", i, err)
		}
		channels = append(channels, channelable)
		logging.FromContext(ctx).Infof("Reconciled Channel Object: %s/%s %+v", s.Namespace, ingressChannelName, channelable)
	}
	s.Status.PropagateChannelStatuses(channels)

	subs := make([]*messagingv1.Subscription, 0, len(s.Spec.Steps))
	for i := 0; i < len(s.Spec.Steps); i++ {
		sub, err := r.reconcileSubscription(ctx, i, s)
		if err != nil {
			s.Status.MarkSubscriptionsNotReady("SubscriptionsNotReady", "Failed to reconcile subscriptions, step: %d", i)
			return fmt.Errorf("failed to reconcile subscription resource for step: %d : %s", i, err)
		}
		subs = append(subs, sub)
		logging.FromContext(ctx).Infof("Reconciled Subscription Object for step: %d: %+v", i, sub)
	}
	s.Status.PropagateSubscriptionStatuses(subs)

	return nil
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, s *v1.Sequence, channelObjRef corev1.ObjectReference) (*eventingduckv1.Channelable, error) {
	logger := logging.FromContext(ctx)
	c, err := r.trackAndFetchChannel(ctx, s, channelObjRef)
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := ducklib.NewPhysicalChannel(
				s.Spec.ChannelTemplate.TypeMeta,
				metav1.ObjectMeta{
					Name:      channelObjRef.Name,
					Namespace: s.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(s),
					},
				},
				ducklib.WithPhysicalChannelSpec(s.Spec.ChannelTemplate.Spec),
			)
			logger.Infof("Creating Channel Object: %+v", newChannel)
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

			channelable := &eventingduckv1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				logger.Errorw("Failed to convert to channelable", zap.Any("channel", created), zap.Error(err))
				return nil, fmt.Errorf("Failed to convert created channel to channelable: %s", err)
			}
			return channelable, nil
		}
		logger.Errorw("Failed to get Channel", zap.Any("channel", channelObjRef), zap.Error(err))
		return nil, err
	}
	logger.Debugw("Found Channel", zap.Any("channel", channelObjRef))
	channelable, ok := c.(*eventingduckv1.Channelable)
	if !ok {
		logger.Errorw("Failed to convert to Channelable Object", zap.Any("channel", c), zap.Error(err))
		return nil, fmt.Errorf("Failed to convert to Channelable Object: %+v", c)
	}
	return channelable, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, step int, p *v1.Sequence) (*messagingv1.Subscription, error) {
	expected := resources.NewSubscription(step, p)

	subName := resources.SequenceSubscriptionName(p.Name, step)
	sub, err := r.subscriptionLister.Subscriptions(p.Namespace).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Infof("Creating subscription: %+v", sub)
		newSub, err := r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Create(ctx, sub, metav1.CreateOptions{})
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequence's subscription failed: %v", err)
			return nil, err
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Errorw("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequences's subscription failed: %v", err)
		return nil, fmt.Errorf("failed to get subscription: %s", err)
	} else if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the subscription. We delete
		// it instead, and re-create it.
		err = r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Delete(ctx, sub.Name, metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Infow("Cannot delete subscription", zap.Error(err))
			return nil, err
		}
		newSub, err := r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			logging.FromContext(ctx).Infow("Cannot create subscription", zap.Error(err))
			return nil, err
		}
		return newSub, nil
	}
	return sub, nil
}

func (r *Reconciler) trackAndFetchChannel(ctx context.Context, seq *v1.Sequence, ref corev1.ObjectReference) (runtime.Object, pkgreconciler.Event) {
	// Track the channel using the channelableTracker.
	// We don't need the explicitly set a channelInformer, as this will dynamically generate one for us.
	// This code needs to be called before checking the existence of the `channel`, in order to make sure the
	// subscription controller will reconcile upon a `channel` change.
	if err := r.channelableTracker.TrackInNamespace(ctx, seq)(ref); err != nil {
		return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, "TrackerFailed", "unable to track changes to channel %+v : %v", ref, err)
	}
	chLister, err := r.channelableTracker.ListerFor(ref)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error getting lister for Channel", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	obj, err := chLister.ByNamespace(seq.Namespace).Get(ref.Name)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error getting channel from lister", zap.Any("channel", ref), zap.Error(err))
		return nil, err
	}
	return obj, err
}
