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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"knative.dev/pkg/kmeta"
	"sort"

	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/flows/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/auth"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	sequencereconciler "knative.dev/eventing/pkg/client/injection/reconciler/flows/v1/sequence"
	eventingv1alpha1listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/flows/v1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/pkg/kmp"

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

	eventPolicyLister eventingv1alpha1listers.EventPolicyLister
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

	featureFlags := feature.FromContext(ctx)

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
			err = fmt.Errorf("failed to reconcile channel %s at step %d: %w", ingressChannelName, i, err)
			s.Status.MarkChannelsNotReady("ChannelsNotReady", err.Error())
			return err
		}
		channels = append(channels, channelable)
		logging.FromContext(ctx).Infof("Reconciled Channel Object: %s/%s %+v", s.Namespace, ingressChannelName, channelable)
	}

	s.Status.PropagateChannelStatuses(channels)

	subs := make([]*messagingv1.Subscription, 0, len(s.Spec.Steps))
	for i := 0; i < len(s.Spec.Steps); i++ {
		sub, err := r.reconcileSubscription(ctx, i, s)
		if err != nil {
			err := fmt.Errorf("failed to reconcile subscription resource for step: %d : %s", i, err)
			s.Status.MarkSubscriptionsNotReady("SubscriptionsNotReady", err.Error())
			return err
		}
		subs = append(subs, sub)
		logging.FromContext(ctx).Infof("Reconciled Subscription Object for step: %d: %+v", i, sub)
	}
	s.Status.PropagateSubscriptionStatuses(subs)

	// If a sequence is modified resulting in the number of steps decreasing, there will be
	// leftover channels and subscriptions that need to be removed.
	if err := r.removeUnwantedChannels(ctx, channelResourceInterface, s, channels); err != nil {
		return err
	}

	if err := r.reconcileEventPolicies(ctx, s, channels, subs, featureFlags); err != nil {
		return fmt.Errorf("failed to reconcile EventPolicies: %w", err)
	}

	err := auth.UpdateStatusWithEventPolicies(featureFlags, &s.Status.AppliedEventPoliciesStatus, &s.Status, r.eventPolicyLister, v1.SchemeGroupVersion.WithKind("Sequence"), s.ObjectMeta)
	if err != nil {
		return fmt.Errorf("could not update Sequence status with EventPolicies: %v", err)
	}

	return r.removeUnwantedSubscriptions(ctx, s, subs)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, s *v1.Sequence, channelObjRef corev1.ObjectReference) (*eventingduckv1.Channelable, error) {
	logger := logging.FromContext(ctx)
	c, err := r.trackAndFetchChannel(ctx, s, channelObjRef)
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := duck.NewPhysicalChannel(
				s.Spec.ChannelTemplate.TypeMeta,
				metav1.ObjectMeta{
					Name:      channelObjRef.Name,
					Namespace: s.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						*kmeta.NewControllerRef(s),
					},
				},
				duck.WithPhysicalChannelSpec(s.Spec.ChannelTemplate.Spec),
			)
			logger.Infof("Creating Channel Object: %+v", newChannel)
			if err != nil {
				return nil, fmt.Errorf("failed to create Channel resource %v: %w", channelObjRef, err)
			}
			created, err := channelResourceInterface.Create(ctx, newChannel, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create channel %v: %w", channelObjRef, err)
			}
			logger.Debugw("Created Channel", zap.Any("channel", newChannel))

			channelable := &eventingduckv1.Channelable{}
			err = duckapis.FromUnstructured(created, channelable)
			if err != nil {
				return nil, fmt.Errorf("failed to convert Channelable %v: %w", created, err)
			}
			return channelable, nil
		}
		return nil, fmt.Errorf("failed to get channel %v: %w", channelObjRef, err)
	}
	logger.Debugw("Found Channel", zap.Any("channel", channelObjRef))
	channelable, ok := c.(*eventingduckv1.Channelable)
	if !ok {
		return nil, fmt.Errorf("failed to convert to Channelable Object %+v: %w", c, err)
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
	} else if immutableFieldsChanged := expected.CheckImmutableFields(ctx, sub); immutableFieldsChanged != nil {
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
	} else if equal, err := kmp.SafeEqual(sub.Spec, expected.Spec); !equal || err != nil {
		// only the mutable fields were changed, so we can update the subscription
		updatedSub, err := r.eventingClientSet.MessagingV1().Subscriptions(sub.Namespace).Update(ctx, expected, metav1.UpdateOptions{})
		if err != nil {
			logging.FromContext(ctx).Infow("Cannot update subscription", zap.Error(err))
			return nil, err
		}
		return updatedSub, nil
	}
	return sub, nil
}

func (r *Reconciler) trackAndFetchChannel(ctx context.Context, seq *v1.Sequence, ref corev1.ObjectReference) (runtime.Object, pkgreconciler.Event) {
	// Track the channel using the channelableTracker.
	// We don't need the explicitly set a channelInformer, as this will dynamically generate one for us.
	// This code needs to be called before checking the existence of the `channel`, in order to make sure the
	// subscription controller will reconcile upon a `channel` change.
	if err := r.channelableTracker.TrackInNamespace(ctx, seq)(ref); err != nil {
		return nil, pkgreconciler.NewEvent(corev1.EventTypeWarning, "TrackerFailed", "unable to track changes to channel %+v : %w", ref, err)
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

func (r *Reconciler) removeUnwantedChannels(ctx context.Context, channelResourceInterface dynamic.ResourceInterface, seq *v1.Sequence, wanted []*eventingduckv1.Channelable) error {
	channelObjRef := corev1.ObjectReference{
		Kind:       seq.Spec.ChannelTemplate.Kind,
		APIVersion: seq.Spec.ChannelTemplate.APIVersion,
	}

	l, err := r.channelableTracker.ListerFor(channelObjRef)
	if err != nil {
		logging.FromContext(ctx).Errorw("Error getting lister for Channel", zap.Any("channelRef", channelObjRef), zap.Error(err))
		return err
	}

	exists, err := l.ByNamespace(seq.GetNamespace()).List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Errorw("Error listing Channels", zap.Any("namespace", seq.Namespace), zap.Any("channelRef", channelObjRef), zap.Error(err))
		return err
	}

	for _, c := range exists {
		ch, err := kmeta.DeletionHandlingAccessor(c)
		if err != nil {
			logging.FromContext(ctx).Errorw("Failed to get channel", zap.Any("channel", c), zap.Error(err))
			return err
		}

		if !ch.GetDeletionTimestamp().IsZero() ||
			!metav1.IsControlledBy(ch, seq) {
			continue
		}

		used := false
		for _, cw := range wanted {
			if cw.Name == ch.GetName() {
				used = true
				break
			}
		}

		if !used {
			err = channelResourceInterface.Delete(ctx, ch.GetName(), metav1.DeleteOptions{})
			if err != nil {
				logging.FromContext(ctx).Errorw("Failed to delete Channel", zap.Any("channel", ch), zap.Error(err))
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) removeUnwantedSubscriptions(ctx context.Context, seq *v1.Sequence, wanted []*messagingv1.Subscription) error {
	subs, err := r.subscriptionLister.Subscriptions(seq.Namespace).List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Errorw("Error listing Subscriptions", zap.Any("namespace", seq.Namespace), zap.Error(err))
		return err
	}

	for _, sub := range subs {
		if !sub.GetDeletionTimestamp().IsZero() ||
			!metav1.IsControlledBy(sub, seq) {
			continue
		}

		used := false
		for _, sw := range wanted {
			if sub.Name == sw.Name {
				used = true
				break
			}
		}

		if !used {
			err = r.eventingClientSet.MessagingV1().Subscriptions(seq.Namespace).Delete(ctx, sub.Name, metav1.DeleteOptions{})
			if err != nil {
				logging.FromContext(ctx).Infow("Failed to delete Subscription", zap.Any("subscription", sub), zap.Error(err))
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) reconcileEventPolicies(ctx context.Context, s *v1.Sequence, channels []*eventingduckv1.Channelable, subs []*messagingv1.Subscription, featureFlags feature.Flags) error {
	if !featureFlags.IsOIDCAuthentication() {
		return r.cleanupAllEventPolicies(ctx, s)
	}

	existingPolicies, err := r.listEventPoliciesForSequence(s)
	if err != nil {
		return fmt.Errorf("failed to list existing EventPolicies: %w", err)
	}

	// Prepare maps for efficient lookups, updates, and deletions of policies
	existingPolicyMap := make(map[string]*eventingv1alpha1.EventPolicy)
	for _, policy := range existingPolicies {
		existingPolicyMap[policy.Name] = policy
	}

	// Prepare lists for different actions so that policies can be categorized
	var policiesToUpdate, policiesToCreate []*eventingv1alpha1.EventPolicy
	policiesToDelete := make([]*eventingv1alpha1.EventPolicy, 0, len(existingPolicies))

	// Handle intermediate channel policies (skip the first channel as it's the input channel!)
	for i := 1; i < len(channels); i++ {
		expectedPolicy := resources.MakeEventPolicyForSequenceChannel(s, channels[i], subs[i-1])
		existingPolicy, exists := existingPolicyMap[expectedPolicy.Name]

		if exists {
			if !equality.Semantic.DeepDerivative(expectedPolicy.Spec, existingPolicy.Spec) {
				policiesToUpdate = append(policiesToUpdate, expectedPolicy)
			}
			delete(existingPolicyMap, expectedPolicy.Name)
		} else {
			policiesToCreate = append(policiesToCreate, expectedPolicy)
		}
	}

	// Handle input channel policies
	inputPolicies, err := r.prepareInputChannelEventPolicy(s, channels[0])
	if err != nil {
		return fmt.Errorf("failed to prepare input channel EventPolicies: %w", err)
	}
	for _, inputPolicy := range inputPolicies {
		existingInputPolicy, exists := existingPolicyMap[inputPolicy.Name]
		if exists {
			if !equality.Semantic.DeepDerivative(inputPolicy.Spec, existingInputPolicy.Spec) {
				policiesToUpdate = append(policiesToUpdate, inputPolicy)
			}
			delete(existingPolicyMap, inputPolicy.Name)
		} else {
			policiesToCreate = append(policiesToCreate, inputPolicy)
		}
	}

	// Any remaining policies in the map should be deleted
	for _, policy := range existingPolicyMap {
		policiesToDelete = append(policiesToDelete, policy)
	}

	// Perform the actual CRUD operations
	if err := r.createEventPolicies(ctx, policiesToCreate); err != nil {
		return fmt.Errorf("failed to create EventPolicies: %w", err)
	}
	if err := r.updateEventPolicies(ctx, policiesToUpdate); err != nil {
		return fmt.Errorf("failed to update EventPolicies: %w", err)
	}
	if err := r.deleteEventPolicies(ctx, policiesToDelete); err != nil {
		return fmt.Errorf("failed to delete EventPolicies: %w", err)
	}

	return nil
}

// listEventPoliciesForSequence lists all EventPolicies (e.g. the policies for the input channel and the intermediate channels) created during reconcileKind that are associated with the given Sequence.
func (r *Reconciler) listEventPoliciesForSequence(s *v1.Sequence) ([]*eventingv1alpha1.EventPolicy, error) {
	labelSelector := labels.SelectorFromSet(map[string]string{
		resources.SequenceChannelEventPolicyLabelPrefix + "sequence-name": s.Name,
	})
	return r.eventPolicyLister.EventPolicies(s.Namespace).List(labelSelector)
}

func (r *Reconciler) prepareInputChannelEventPolicy(s *v1.Sequence, inputChannel *eventingduckv1.Channelable) ([]*eventingv1alpha1.EventPolicy, error) {
	matchingPolicies, err := auth.GetEventPoliciesForResource(
		r.eventPolicyLister,
		v1.SchemeGroupVersion.WithKind("Sequence"),
		s.ObjectMeta,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get matching EventPolicies for Sequence: %w", err)
	}

	if len(matchingPolicies) == 0 {
		return nil, nil
	}

	sort.Slice(matchingPolicies, func(i, j int) bool {
		return matchingPolicies[i].Name < matchingPolicies[j].Name
	})

	var inputChannelPolicies []*eventingv1alpha1.EventPolicy
	for _, policy := range matchingPolicies {
		inputChannelPolicy := resources.MakeEventPolicyForSequenceInputChannel(s, inputChannel, policy)
		inputChannelPolicies = append(inputChannelPolicies, inputChannelPolicy)
	}


	return inputChannelPolicies, nil
}

func (r *Reconciler) createEventPolicies(ctx context.Context, policies []*eventingv1alpha1.EventPolicy) error {
	for _, policy := range policies {
		_, err := r.eventingClientSet.EventingV1alpha1().EventPolicies(policy.Namespace).Create(ctx, policy, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) updateEventPolicies(ctx context.Context, policies []*eventingv1alpha1.EventPolicy) error {
	for _, policy := range policies {
		_, err := r.eventingClientSet.EventingV1alpha1().EventPolicies(policy.Namespace).Update(ctx, policy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) deleteEventPolicies(ctx context.Context, policies []*eventingv1alpha1.EventPolicy) error {
	for _, policy := range policies {
		err := r.eventingClientSet.EventingV1alpha1().EventPolicies(policy.Namespace).Delete(ctx, policy.Name, metav1.DeleteOptions{})
		if err != nil && !apierrs.IsNotFound(err) {
			return err
		}
	}
	return nil
}

func (r *Reconciler) cleanupAllEventPolicies(ctx context.Context, s *v1.Sequence) error {
	policies, err := r.listEventPoliciesForSequence(s)
	if err != nil {
		return fmt.Errorf("failed to list EventPolicies for cleanup: %w", err)
	}
	return r.deleteEventPolicies(ctx, policies)
}
