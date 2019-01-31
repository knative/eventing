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

package trigger

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/knative/eventing/pkg/provisioners"
	"k8s.io/apimachinery/pkg/types"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/logging"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	finalizerName = controllerAgentName

	eventTypeKey = "eventing.knative.dev/broker/eventType"

	// Name of the corev1.Events emitted from the reconciliation process
	triggerReconciled         = "TriggerReconciled"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	trigger := &v1alpha1.Trigger{}
	err := r.client.Get(context.TODO(), request.NamespacedName, trigger)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Trigger")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not Get Trigger", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Reconcile this copy of the Trigger and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, trigger)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Trigger", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Trigger reconciled")
		r.recorder.Event(trigger, corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled")
	}

	if _, err = r.updateStatus(trigger.DeepCopy()); err != nil {
		logging.FromContext(ctx).Error("Failed to update Trigger status", zap.Error(err))
		r.recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, t *v1alpha1.Trigger) error {
	t.Status.InitializeConditions()

	// 1. Verify the Broker exists.
	// 2. Determine subscribers.
	// 3. Look over all subscribable resources and subscribe to the correct ones.
	// 4. [Do not do for the prototype] Inject a filter on every subscription.

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		r.removeFromTriggers(t)
		provisioners.RemoveFinalizer(t, finalizerName)
		return nil
	}

	provisioners.AddFinalizer(t, finalizerName)
	r.AddToTriggers(t)

	broker, err := r.getBroker(ctx, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		t.Status.MarkBrokerDoesNotExists()
		return err
	}
	t.Status.MarkBrokerExists()

	subscribables, err := r.getRelevantSubscribables(ctx, t, broker.Spec.Selector)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get relevant subscribables", zap.Error(err))
		return err
	}

	err = r.subscribeAll(ctx, t, subscribables)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("notSubscribed", "%v", err)
		return err
	}
	t.Status.MarkSubscribed()

	return nil
}

func (r *reconciler) AddToTriggers(t *v1alpha1.Trigger) {
	name := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: t.Namespace,
			Name:      t.Name,
		},
	}

	// We will be reconciling an already existing Trigger far more often than adding a new one, so
	// check with a read lock before using the write lock.
	r.triggersLock.RLock()
	triggersInNamespace := r.triggers[t.Namespace]
	var present bool
	if triggersInNamespace != nil {
		_, present = triggersInNamespace[name]
	} else {
		present = false
	}
	r.triggersLock.RUnlock()

	if present {
		// Already present in the map.
		return
	}

	r.triggersLock.Lock()
	triggersInNamespace = r.triggers[t.Namespace]
	if triggersInNamespace == nil {
		r.triggers[t.Namespace] = make(map[reconcile.Request]bool)
		triggersInNamespace = r.triggers[t.Namespace]
	}
	triggersInNamespace[name] = false
	r.triggersLock.Unlock()
}

func (r *reconciler) removeFromTriggers(t *v1alpha1.Trigger) {
	name := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: t.Namespace,
			Name:      t.Name,
		},
	}

	r.triggersLock.Lock()
	triggersInNamespace := r.triggers[t.Namespace]
	if triggersInNamespace != nil {
		delete(triggersInNamespace, name)
	}
	r.triggersLock.Unlock()
}

// updateStatus may in fact update the trigger's finalizers in addition to the status
func (r *reconciler) updateStatus(trigger *v1alpha1.Trigger) (*v1alpha1.Trigger, error) {
	objectKey := client.ObjectKey{Namespace: trigger.Namespace, Name: trigger.Name}
	latestTrigger := &v1alpha1.Trigger{}

	if err := r.client.Get(context.TODO(), objectKey, latestTrigger); err != nil {
		return nil, err
	}

	triggerChanged := false

	if !equality.Semantic.DeepEqual(latestTrigger.Finalizers, trigger.Finalizers) {
		latestTrigger.SetFinalizers(trigger.ObjectMeta.Finalizers)
		if err := r.client.Update(context.TODO(), latestTrigger); err != nil {
			return nil, err
		}
		triggerChanged = true
	}

	if equality.Semantic.DeepEqual(latestTrigger.Status, trigger.Status) {
		return latestTrigger, nil
	}

	if triggerChanged {
		// Refetch
		latestTrigger = &v1alpha1.Trigger{}
		if err := r.client.Get(context.TODO(), objectKey, latestTrigger); err != nil {
			return nil, err
		}
	}

	latestTrigger.Status = trigger.Status
	if err := r.client.Status().Update(context.TODO(), latestTrigger); err != nil {
		return nil, err
	}

	return latestTrigger, nil
}

func (r *reconciler) getBroker(ctx context.Context, t *v1alpha1.Trigger) (*v1alpha1.Broker, error) {
	broker := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Spec.Broker,
	}
	err := r.client.Get(ctx, name, broker)
	return broker, err
}

func (r *reconciler) getRelevantSubscribables(ctx context.Context, t *v1alpha1.Trigger, selector *metav1.LabelSelector) ([]v1alpha1.Channel, error) {
	selector.MatchLabels[eventTypeKey] = t.Spec.Type

	subscribables := make([]v1alpha1.Channel, 0)
	opts := &client.ListOptions{
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
			},
		},
		Namespace: t.Namespace,
	}
	for {
		list := v1alpha1.ChannelList{}
		err := r.client.List(ctx, opts, &list)
		if err != nil {
			return nil, err
		}
		for _, s := range list.Items {
			subscribables = append(subscribables, s)
			if list.Continue != "" {
				opts.Raw.Continue = list.Continue
			} else {
				return subscribables, nil
			}
		}
	}
}

func (r *reconciler) subscribeAll(ctx context.Context, t *v1alpha1.Trigger, subscribables []v1alpha1.Channel) error {
	for _, subscribable := range subscribables {
		_, err := r.subscribe(ctx, t, &subscribable)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *reconciler) subscribe(ctx context.Context, t *v1alpha1.Trigger, subscribable *v1alpha1.Channel) (*v1alpha1.Subscription, error) {
	expected := makeSubscription(t, subscribable)

	sub, err := r.getSubscription(ctx, t, subscribable)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		sub = expected
		err = r.client.Create(ctx, sub)
		if err != nil {
			return nil, err
		}
		return sub, nil
	} else if err != nil {
		return nil, err
	}

	// Update Subscription if it has changed.
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		sub.Spec = expected.Spec
		err = r.client.Update(ctx, sub)
		if err != nil {
			return nil, err
		}
	}
	return sub, nil
}

func (r *reconciler) getSubscription(ctx context.Context, t *v1alpha1.Trigger, subscribable *v1alpha1.Channel) (*v1alpha1.Subscription, error) {
	list := &v1alpha1.SubscriptionList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     t.Namespace,
		LabelSelector: labels.SelectorFromSet(subscriptionLabels(t)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Subscription",
			},
		},
	}

	err := r.client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, s := range list.Items {
		if metav1.IsControlledBy(&s, t) {
			return &s, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func makeSubscription(t *v1alpha1.Trigger, subscribable *v1alpha1.Channel) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    t.Namespace,
			GenerateName: fmt.Sprintf("%s-%s-", t.Spec.Broker, t.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   t.GroupVersionKind().Group,
					Version: t.GroupVersionKind().Version,
					Kind:    t.GroupVersionKind().Kind,
				}),
			},
			Labels: subscriptionLabels(t),
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: subscribable.APIVersion,
				Kind:       subscribable.Kind,
				Namespace:  subscribable.Namespace,
				Name:       subscribable.Name,
			},
			Subscriber: t.Spec.Subscriber,
		},
	}
}

func subscriptionLabels(t *v1alpha1.Trigger) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":  t.Spec.Broker,
		"eventing.knative.dev/trigger": t.Name,
	}
}
