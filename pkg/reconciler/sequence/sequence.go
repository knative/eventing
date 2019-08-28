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
	"errors"
	"fmt"
	"reflect"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	duckroot "knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"

	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/sequence/resources"
	"knative.dev/eventing/pkg/utils"
)

const (
	reconciled         = "Reconciled"
	reconcileFailed    = "ReconcileFailed"
	updateStatusFailed = "UpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	sequenceLister     listers.SequenceLister
	tracker            tracker.Interface
	resourceTracker    duck.ResourceTracker
	subscriptionLister listers.SubscriptionLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// reconcile the two. It then updates the Status block of the Sequence resource
// with the current Status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logging.FromContext(ctx).Debug("reconciling", zap.String("key", key))
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key", zap.Error(err))
		return nil
	}

	// Get the Sequence resource with this namespace/name
	original, err := r.sequenceLister.Sequences(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("Sequence key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	sequence := original.DeepCopy()

	// Reconcile this copy of the Sequence and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, sequence)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Sequence", zap.Error(reconcileErr))
		r.Recorder.Eventf(sequence, corev1.EventTypeWarning, reconcileFailed, "Sequence reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Successfully reconciled Sequence")
		r.Recorder.Eventf(sequence, corev1.EventTypeNormal, reconciled, "Sequence reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, sequence); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Error updating Sequence status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(sequence, corev1.EventTypeWarning, updateStatusFailed, "Failed to update sequence status: %s", key)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, p *v1alpha1.Sequence) error {
	p.Status.InitializeConditions()

	// Reconciling sequence is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole sequence
	// 2. For each of the Steps, create a Subscription to the previous Channel
	//    (hence the first step above for the first step in the "steps"), where the Subscriber points to the
	//    Step, and create intermediate channel for feeding the Reply to (if we allow Reply to be something else
	//    than channel, we could just (optionally) feed it directly to the following subscription.
	// 3. Rinse and repeat step #2 above for each Step in the list
	// 4. If there's a Reply, then the last Subscription will be configured to send the reply to that.
	if p.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	channelResourceInterface := r.DynamicClientSet.Resource(duckroot.KindToResource(p.Spec.ChannelTemplate.GetObjectKind().GroupVersionKind())).Namespace(p.Namespace)

	if channelResourceInterface == nil {
		msg := fmt.Sprintf("Unable to create dynamic client for: %+v", p.Spec.ChannelTemplate)
		logging.FromContext(ctx).Error(msg)
		return errors.New(msg)
	}

	// Tell tracker to reconcile this Sequence whenever my channels change.
	track := r.resourceTracker.TrackInNamespace(p)

	channels := make([]*duckv1alpha1.Channelable, 0, len(p.Spec.Steps))
	for i := 0; i < len(p.Spec.Steps); i++ {
		ingressChannelName := resources.SequenceChannelName(p.Name, i)
		c, err := r.reconcileChannel(ctx, ingressChannelName, channelResourceInterface, p)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", p.Namespace, ingressChannelName), zap.Error(err))
			return err

		}
		// Convert to Channel duck so that we can treat all Channels the same.
		channelable := &duckv1alpha1.Channelable{}
		err = duckapis.FromUnstructured(c, channelable)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", p.Namespace, ingressChannelName), zap.Error(err))
			return err

		}
		// Track channels and enqueue sequence when they change.
		channels = append(channels, channelable)
		if err = track(utils.ObjectRef(channelable, channelable.GroupVersionKind())); err != nil {
			logging.FromContext(ctx).Error("Unable to track changes to Channel", zap.Error(err))
			return err
		}
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Channel Object: %s/%s %+v", p.Namespace, ingressChannelName, c))
	}
	p.Status.PropagateChannelStatuses(channels)

	subs := make([]*v1alpha1.Subscription, 0, len(p.Spec.Steps))
	for i := 0; i < len(p.Spec.Steps); i++ {
		sub, err := r.reconcileSubscription(ctx, i, p)
		if err != nil {
			return fmt.Errorf("Failed to reconcile Subscription Object for step: %d : %s", i, err)
		}
		subs = append(subs, sub)
		logging.FromContext(ctx).Debug(fmt.Sprintf("Reconciled Subscription Object for step: %d: %+v", i, sub))
	}
	p.Status.PropagateSubscriptionStatuses(subs)

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Sequence) (*v1alpha1.Sequence, error) {
	p, err := r.sequenceLister.Sequences(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	// If there's nothing to update, just return.
	if reflect.DeepEqual(p.Status, desired.Status) {
		return p, nil
	}

	// Don't modify the informers copy.
	existing := p.DeepCopy()
	existing.Status = desired.Status

	return r.EventingClientSet.MessagingV1alpha1().Sequences(desired.Namespace).UpdateStatus(existing)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, p *v1alpha1.Sequence) (*unstructured.Unstructured, error) {
	c, err := channelResourceInterface.Get(channelName, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			newChannel, err := resources.NewChannel(channelName, p)
			logging.FromContext(ctx).Error(fmt.Sprintf("Creating Channel Object: %+v", newChannel))
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel resource object: %s/%s", p.Namespace, channelName), zap.Error(err))
				return nil, err
			}
			created, err := channelResourceInterface.Create(newChannel, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error(fmt.Sprintf("Failed to create Channel: %s/%s", p.Namespace, channelName), zap.Error(err))
				return nil, err
			}
			logging.FromContext(ctx).Info(fmt.Sprintf("Created Channel: %s/%s", p.Namespace, channelName), zap.Any("NewChannel", newChannel))
			return created, nil
		}
		logging.FromContext(ctx).Error(fmt.Sprintf("Failed to get Channel: %s/%s", p.Namespace, channelName), zap.Error(err))
		return nil, err
	}
	logging.FromContext(ctx).Debug(fmt.Sprintf("Found Channel: %s/%s", p.Namespace, channelName), zap.Any("NewChannel", c))
	return c, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, step int, p *v1alpha1.Sequence) (*v1alpha1.Subscription, error) {
	expected := resources.NewSubscription(step, p)

	subName := resources.SequenceSubscriptionName(p.Name, step)
	sub, err := r.subscriptionLister.Subscriptions(p.Namespace).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating subscription: %+v", sub))
		newSub, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequence's subscription failed: %v", err)
			return nil, fmt.Errorf("Failed to create Subscription Object for step: %d : %s", step, err)
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Sequences's subscription failed: %v", err)
		return nil, fmt.Errorf("Failed to get subscription: %s", err)
	}
	return sub, nil
}
