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

package broker

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/knative/eventing/pkg/controller/eventing/broker/resources"

	servingv1alpha1 "github.com/knative/serving/pkg/apis/serving/v1alpha1"

	"github.com/knative/eventing/pkg/provisioners/gcppubsub/util/logging"
	"go.uber.org/zap"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	brokerReconciled         = "BrokerReconciled"
	brokerUpdateStatusFailed = "BrokerUpdateStatusFailed"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Broker resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	broker := &v1alpha1.Broker{}
	err := r.client.Get(context.TODO(), request.NamespacedName, broker)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Broker")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not Get Broker", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Reconcile this copy of the Broker and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, broker)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Broker", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("Broker reconciled")
		r.recorder.Event(broker, corev1.EventTypeNormal, brokerReconciled, "Broker reconciled")
	}

	if _, err := r.updateStatus(broker.DeepCopy()); err != nil {
		logging.FromContext(ctx).Error("Failed to update Broker status", zap.Error(err))
		r.recorder.Eventf(broker, corev1.EventTypeWarning, brokerUpdateStatusFailed, "Failed to update Broker's status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, b *v1alpha1.Broker) error {
	b.Status.InitializeConditions()

	// 1. Create the router.
	// 2. Create the activator.
	// 3. Create the filter [do not do for the prototype].
	// 4. Create the 'needs-activation' Channel.
	// 5. Create Subscription from 'needs-activation' Channel to the activator.

	if b.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	// TODO Actually check this, for now the webhook ensures they are identical.
	b.Status.MarkChannelTemplateMatchesSelector()

	dontExist, err := r.verifyResourcesExist(ctx, b.Spec.SubscribableResources)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to determine if resources exist", zap.Error(err))
		return err
	} else if len(dontExist) > 0 {
		b.Status.MarkSubscribableResourcesDoNotExist(dontExist)
	} else {
		b.Status.MarkSubscribableResourcesExist()
	}

	activator, err := r.reconcileActivator(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling activator", zap.Error(err))
		return err
	}

	// TODO Add the filter reconciliation here.

	c, err := r.reconcileNeedsActivationChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the needs activation channel", zap.Error(err))
		return err
	}

	_, err = r.reconcileNeedsActivationSubscription(ctx, b, activator, c)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the needs activation subscription", zap.Error(err))
		return err
	}
	b.Status.MarkRouterAndActivatorExist()

	router, err := r.reconcileRouter(ctx, b, c)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling router", zap.Error(err))
		return err
	}
	b.Status.SetAddress(router.Status.Address.Hostname)

	return nil
}

// updateStatus may in fact update the broker's finalizers in addition to the status
func (r *reconciler) updateStatus(broker *v1alpha1.Broker) (*v1alpha1.Broker, error) {
	objectKey := client.ObjectKey{Namespace: broker.Namespace, Name: broker.Name}
	latestBroker := &v1alpha1.Broker{}

	if err := r.client.Get(context.TODO(), objectKey, latestBroker); err != nil {
		return nil, err
	}

	brokerChanged := false

	if !equality.Semantic.DeepEqual(latestBroker.Finalizers, broker.Finalizers) {
		latestBroker.SetFinalizers(broker.ObjectMeta.Finalizers)
		if err := r.client.Update(context.TODO(), latestBroker); err != nil {
			return nil, err
		}
		brokerChanged = true
	}

	if equality.Semantic.DeepEqual(latestBroker.Status, broker.Status) {
		return latestBroker, nil
	}

	if brokerChanged {
		// Refetch
		latestBroker = &v1alpha1.Broker{}
		if err := r.client.Get(context.TODO(), objectKey, latestBroker); err != nil {
			return nil, err
		}
	}

	latestBroker.Status = broker.Status
	if err := r.client.Status().Update(context.TODO(), latestBroker); err != nil {
		return nil, err
	}

	return latestBroker, nil
}

func (r *reconciler) verifyResourcesExist(ctx context.Context, resources []metav1.GroupVersionKind) ([]metav1.GroupVersionKind, error) {
	dontExist := make([]metav1.GroupVersionKind, 0, len(resources))
	// TODO Implement, for now the webhook asserts it is only Channel, which we assume exists.
	return dontExist, nil
}

func (r *reconciler) reconcileActivator(ctx context.Context, b *v1alpha1.Broker) (*servingv1alpha1.Service, error) {
	expected, err := resources.MakeActivator(&resources.ActivatorArgs{
		Broker:             b,
		Image:              r.activatorImage,
		ServiceAccountName: r.activatorServiceAccountName,
	})
	if err != nil {
		return nil, err
	}
	return r.reconcileKSvc(ctx, expected)
}

func (r *reconciler) reconcileNeedsActivationChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	expected := newNeedsActivationChannel(b)

	c, err := r.getNeedsActivationChannel(ctx, b)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		c = expected
		err = r.client.Create(ctx, c)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if err != nil {
		return nil, err
	}

	// Update Channel if it has changed. Note that we need to both ignore the real Channel's
	// subscribable section and if we need to update the real Channel, retain it.
	expected.Spec.Subscribable = c.Spec.Subscribable
	if !equality.Semantic.DeepDerivative(expected.Spec, c.Spec) {
		c.Spec = expected.Spec
		err = r.client.Update(ctx, c)
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

func (r *reconciler) getNeedsActivationChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	list := &v1alpha1.ChannelList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: labels.SelectorFromSet(needsActivationLabels(b)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
			},
		},
	}

	err := r.client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, b) {
			return &c, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func newNeedsActivationChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	var spec v1alpha1.ChannelSpec
	if b.Spec.ChannelTemplate != nil && b.Spec.ChannelTemplate.Spec != nil {
		spec = *b.Spec.ChannelTemplate.Spec
	}

	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    b.Namespace,
			GenerateName: fmt.Sprintf("%s-broker-needs-activation-", b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b, schema.GroupVersionKind{
					Group:   b.GroupVersionKind().Group,
					Version: b.GroupVersionKind().Version,
					Kind:    b.GroupVersionKind().Kind,
				}),
			},
		},
		Spec: spec,
	}
}

func needsActivationLabels(b *v1alpha1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":                 b.Name,
		"eventing.knative.dev/broker/needsActivation": "true",
	}
}

func (r *reconciler) reconcileNeedsActivationSubscription(ctx context.Context, b *v1alpha1.Broker, activator *servingv1alpha1.Service, c *v1alpha1.Channel) (*v1alpha1.Subscription, error) {
	expected := newNeedsActivationSubscription(b, c, activator)

	sub, err := r.getNeedsActivationSubscription(ctx, b)
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

func (r *reconciler) getNeedsActivationSubscription(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Subscription, error) {
	list := &v1alpha1.SubscriptionList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: labels.SelectorFromSet(needsActivationLabels(b)),
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
	for _, sub := range list.Items {
		if metav1.IsControlledBy(&sub, b) {
			return &sub, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func newNeedsActivationSubscription(b *v1alpha1.Broker, c *v1alpha1.Channel, activator *servingv1alpha1.Service) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    b.Namespace,
			GenerateName: fmt.Sprintf("%s-broker-needs-activation-", b.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b, schema.GroupVersionKind{
					Group:   b.GroupVersionKind().Group,
					Version: b.GroupVersionKind().Version,
					Kind:    b.GroupVersionKind().Kind,
				}),
			},
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: c.APIVersion,
				Kind:       c.Kind,
				Namespace:  c.Namespace,
				Name:       c.Name,
				UID:        c.UID,
			},
			Subscriber: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: activator.APIVersion,
					Kind:       activator.Kind,
					Namespace:  activator.Namespace,
					Name:       activator.Name,
					UID:        activator.UID,
				},
			},
		},
	}
}

func (r *reconciler) reconcileKSvc(ctx context.Context, ksvc *servingv1alpha1.Service) (*servingv1alpha1.Service, error) {
	name := types.NamespacedName{
		Namespace: ksvc.Namespace,
		Name:      ksvc.Name,
	}
	current := &servingv1alpha1.Service{}
	err := r.client.Get(ctx, name, current)
	if k8serrors.IsNotFound(err) {
		err = r.client.Create(ctx, ksvc)
		if err != nil {
			return nil, err
		}
		return ksvc, nil
	} else if err != nil {
		return nil, err
	}

	if !equality.Semantic.DeepDerivative(ksvc.Spec, current.Spec) {
		current.Spec = ksvc.Spec
		err = r.client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func (r *reconciler) reconcileRouter(ctx context.Context, b *v1alpha1.Broker, c *v1alpha1.Channel) (*servingv1alpha1.Service, error) {
	expected, err := resources.MakeRouter(&resources.RouterArgs{
		Broker:              b,
		Image:               r.routerImage,
		ServiceAccountName:  r.routerServiceAccountName,
		NeedsActivationHost: c.Status.Address.Hostname,
	})
	if err != nil {
		return nil, err
	}
	return r.reconcileKSvc(ctx, expected)
}
