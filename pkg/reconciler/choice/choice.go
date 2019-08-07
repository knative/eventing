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

package choice

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
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/choice/resources"
	"knative.dev/eventing/pkg/utils"
	duckroot "knative.dev/pkg/apis"
	duckapis "knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/tracker"
)

const (
	reconciled         = "Reconciled"
	reconcileFailed    = "ReconcileFailed"
	updateStatusFailed = "UpdateStatusFailed"
)

type Reconciler struct {
	*reconciler.Base

	// listers index properties about resources
	choiceLister       listers.ChoiceLister
	tracker            tracker.Interface
	resourceTracker    duck.ResourceTracker
	subscriptionLister eventinglisters.SubscriptionLister
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// reconcile the two. It then updates the Status block of the Choice resource
// with the current Status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logging.FromContext(ctx).Debug("reconciling", zap.String("key", key))
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key", zap.Error(err))
		return nil
	}

	// Get the Choice resource with this namespace/name
	original, err := r.choiceLister.Choices(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("Choice key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy
	choice := original.DeepCopy()

	// Reconcile this copy of the Choice and then write back any status
	// updates regardless of whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, choice)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Choice", zap.Error(reconcileErr))
		r.Recorder.Eventf(choice, corev1.EventTypeWarning, reconcileFailed, "Choice reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Successfully reconciled Choice")
		r.Recorder.Eventf(choice, corev1.EventTypeNormal, reconciled, "Choice reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, choice); updateStatusErr != nil {
		logging.FromContext(ctx).Warn("Error updating Choice status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(choice, corev1.EventTypeWarning, updateStatusFailed, "Failed to update choice status: %s", key)
		return updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, p *v1alpha1.Choice) error {
	p.Status.InitializeConditions()

	// Reconciling choice is pretty straightforward, it does the following things:
	// 1. Create a channel fronting the whole choice and one filter channel per case.
	// 2. For each of the Cases:
	//     2.1 create a Subscription to the fronting Channel, subscribe the filter and send reply to the filter Channel
	//     2.2 create a Subscription to the filter Channel, subcribe the subscriber and send reply to
	//         either the case Reply. If not present, send reply to the global Reply. If not present, do not send reply.
	// 3. Rinse and repeat step #2 above for each Case in the list
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

	// Tell tracker to reconcile this Choice whenever my channels change.
	track := r.resourceTracker.TrackInNamespace(p)

	var ingressChannel *duckv1alpha1.Channelable
	channels := make([]*duckv1alpha1.Channelable, 0, len(p.Spec.Cases))
	for i := -1; i < len(p.Spec.Cases); i++ {
		var channelName string
		if i == -1 {
			channelName = resources.ChoiceChannelName(p.Name)
		} else {
			channelName = resources.ChoiceCaseChannelName(p.Name, i)
		}

		c, err := r.reconcileChannel(ctx, channelName, channelResourceInterface, p)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to reconcile Channel Object: %s/%s", p.Namespace, channelName), zap.Error(err))
			return err

		}
		// Convert to Channel duck so that we can treat all Channels the same.
		channelable := &duckv1alpha1.Channelable{}
		err = duckapis.FromUnstructured(c, channelable)
		if err != nil {
			logging.FromContext(ctx).Error(fmt.Sprintf("Failed to convert to Channelable Object: %s/%s", p.Namespace, channelName), zap.Error(err))
			return err

		}
		// Track channels and enqueue choice when they change.
		if err = track(utils.ObjectRef(channelable, channelable.GroupVersionKind())); err != nil {
			logging.FromContext(ctx).Error("Unable to track changes to Channel", zap.Error(err))
			return err
		}
		logging.FromContext(ctx).Info(fmt.Sprintf("Reconciled Channel Object: %s/%s %+v", p.Namespace, channelName, c))

		if i == -1 {
			ingressChannel = channelable
		} else {
			channels = append(channels, channelable)
		}
	}
	p.Status.PropagateChannelStatuses(ingressChannel, channels)

	filterSubs := make([]*eventingv1alpha1.Subscription, 0, len(p.Spec.Cases))
	subs := make([]*eventingv1alpha1.Subscription, 0, len(p.Spec.Cases))
	for i := 0; i < len(p.Spec.Cases); i++ {
		filterSub, sub, err := r.reconcileCase(ctx, i, p)
		if err != nil {
			return fmt.Errorf("Failed to reconcile Subscription Objects for case: %d : %s", i, err)
		}
		subs = append(subs, sub)
		filterSubs = append(filterSubs, filterSub)
		logging.FromContext(ctx).Debug(fmt.Sprintf("Reconciled Subscription Objects for case: %d: %+v, %+v", i, filterSub, sub))
	}
	p.Status.PropagateSubscriptionStatuses(filterSubs, subs)

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Choice) (*v1alpha1.Choice, error) {
	p, err := r.choiceLister.Choices(desired.Namespace).Get(desired.Name)
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

	return r.EventingClientSet.MessagingV1alpha1().Choices(desired.Namespace).UpdateStatus(existing)
}

func (r *Reconciler) reconcileChannel(ctx context.Context, channelName string, channelResourceInterface dynamic.ResourceInterface, p *v1alpha1.Choice) (*unstructured.Unstructured, error) {
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

func (r *Reconciler) reconcileCase(ctx context.Context, caseNumber int, p *v1alpha1.Choice) (*eventingv1alpha1.Subscription, *eventingv1alpha1.Subscription, error) {
	filterExpected := resources.NewFilterSubscription(caseNumber, p)
	filterSubName := resources.ChoiceFilterSubscriptionName(p.Name, caseNumber)

	filterSub, err := r.reconcileSubscription(ctx, caseNumber, filterExpected, filterSubName, p.Namespace)
	if err != nil {
		return nil, nil, err
	}

	expected := resources.NewSubscription(caseNumber, p)
	subName := resources.ChoiceSubscriptionName(p.Name, caseNumber)

	sub, err := r.reconcileSubscription(ctx, caseNumber, expected, subName, p.Namespace)
	if err != nil {
		return nil, nil, err
	}

	return filterSub, sub, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, caseNumber int, expected *eventingv1alpha1.Subscription, subName, ns string) (*eventingv1alpha1.Subscription, error) {
	sub, err := r.subscriptionLister.Subscriptions(ns).Get(subName)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info(fmt.Sprintf("Creating subscription: %+v", sub))
		newSub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			// TODO: Send events here, or elsewhere?
			//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Choice's subscription failed: %v", err)
			return nil, fmt.Errorf("Failed to create Subscription Object for case: %d : %s", caseNumber, err)
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		// TODO: Send events here, or elsewhere?
		//r.Recorder.Eventf(p, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Choices's subscription failed: %v", err)
		return nil, fmt.Errorf("Failed to get subscription: %s", err)
	}
	return sub, nil
}
