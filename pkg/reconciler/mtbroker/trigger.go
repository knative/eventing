/*
Copyright 2020 The Knative Authors

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

package mtbroker

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/eventing/pkg/reconciler/trigger/path"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/system"
)

const (
	// Name of the corev1.Events emitted from the Trigger reconciliation process.
	triggerReconciled         = "TriggerReconciled"
	triggerReadinessChanged   = "TriggerReadinessChanged"
	triggerReconcileFailed    = "TriggerReconcileFailed"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
	subscriptionDeleteFailed  = "SubscriptionDeleteFailed"
	subscriptionCreateFailed  = "SubscriptionCreateFailed"
	subscriptionGetFailed     = "SubscriptionGetFailed"
	triggerChannelFailed      = "TriggerChannelFailed"
	triggerServiceFailed      = "TriggerServiceFailed"
)

func (r *Reconciler) reconcileTrigger(ctx context.Context, b *v1alpha1.Broker, t *v1alpha1.Trigger) error {
	t.Status.InitializeConditions()

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	t.Status.PropagateBrokerStatus(&b.Status)

	brokerTrigger := b.Status.TriggerChannel
	if brokerTrigger == nil {
		// Should not happen because Broker is ready to go if we get here
		return errors.New("failed to find Broker's Trigger channel")
	}

	if t.Spec.Subscriber.Ref != nil {
		// To call URIFromDestination(dest apisv1alpha1.Destination, parent interface{}), dest.Ref must have a Namespace
		// We will use the Namespace of Trigger as the Namespace of dest.Ref
		t.Spec.Subscriber.Ref.Namespace = t.GetNamespace()
	}

	subscriberURI, err := r.uriResolver.URIFromDestinationV1(t.Spec.Subscriber, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		t.Status.MarkSubscriberResolvedFailed("Unable to get the Subscriber's URI", "%v", err)
		t.Status.SubscriberURI = nil
		return err
	}
	t.Status.SubscriberURI = subscriberURI
	t.Status.MarkSubscriberResolvedSucceeded()

	sub, err := r.subscribeToBrokerChannel(ctx, b, t, brokerTrigger)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("NotSubscribed", "%v", err)
		return err
	}
	t.Status.PropagateSubscriptionStatus(&sub.Status)

	if err := r.checkDependencyAnnotation(ctx, t, b); err != nil {
		return err
	}

	return nil
}

// subscribeToBrokerChannel subscribes service 'svc' to the Broker's channels.
func (r *Reconciler) subscribeToBrokerChannel(ctx context.Context, b *v1alpha1.Broker, t *v1alpha1.Trigger, brokerTrigger *corev1.ObjectReference) (*messagingv1alpha1.Subscription, error) {
	uri := &apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName("broker-filter", system.Namespace()),
		Path:   path.Generate(t),
	}
	// Note that we have to hard code the brokerGKV stuff as sometimes typemeta is not
	// filled in. So instead of b.TypeMeta.Kind and b.TypeMeta.APIVersion, we have to
	// do it this way.
	brokerObjRef := &corev1.ObjectReference{
		Kind:       brokerGVK.Kind,
		APIVersion: brokerGVK.GroupVersion().String(),
		Name:       b.Name,
		Namespace:  b.Namespace,
	}
	expected := resources.NewSubscription(t, brokerTrigger, brokerObjRef, uri, b.Spec.Delivery)

	sub, err := r.subscriptionLister.Subscriptions(t.Namespace).Get(expected.Name)
	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		logging.FromContext(ctx).Info("Creating subscription")
		sub, err = r.EventingClientSet.MessagingV1alpha1().Subscriptions(t.Namespace).Create(expected)
		if err != nil {
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
			return nil, err
		}
		return sub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionGetFailed, "Getting the Trigger's Subscription failed: %v", err)
		return nil, err
	} else if !metav1.IsControlledBy(sub, t) {
		t.Status.MarkSubscriptionNotOwned(sub)
		return nil, fmt.Errorf("trigger %q does not own subscription %q", t.Name, sub.Name)
	} else if sub, err = r.reconcileSubscription(ctx, t, expected, sub); err != nil {
		logging.FromContext(ctx).Error("Failed to reconcile subscription", zap.Error(err))
		return sub, err
	}

	return sub, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, t *v1alpha1.Trigger, expected, actual *messagingv1alpha1.Subscription) (*messagingv1alpha1.Subscription, error) {
	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = actual.Spec.DeprecatedGeneration
	if equality.Semantic.DeepDerivative(expected.Spec, actual.Spec) {
		return actual, nil
	}
	logging.FromContext(ctx).Info("Differing Subscription", zap.Any("expected", expected.Spec), zap.Any("actual", actual.Spec))

	// Given that spec.channel is immutable, we cannot just update the Subscription. We delete
	// it and re-create it instead.
	logging.FromContext(ctx).Info("Deleting subscription", zap.String("namespace", actual.Namespace), zap.String("name", actual.Name))
	err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(t.Namespace).Delete(actual.Name, &metav1.DeleteOptions{})
	if err != nil {
		logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
		r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionDeleteFailed, "Delete Trigger's subscription failed: %v", err)
		return nil, err
	}
	logging.FromContext(ctx).Info("Creating subscription")
	newSub, err := r.EventingClientSet.MessagingV1alpha1().Subscriptions(t.Namespace).Create(expected)
	if err != nil {
		logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
		r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
		return nil, err
	}
	return newSub, nil
}

func (r *Reconciler) updateTriggerStatus(ctx context.Context, desired *v1alpha1.Trigger) (*v1alpha1.Trigger, error) {
	trigger, err := r.triggerLister.Triggers(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(trigger.Status, desired.Status) {
		return trigger, nil
	}

	becomesReady := desired.Status.IsReady() && !trigger.Status.IsReady()

	// Don't modify the informers copy.
	existing := trigger.DeepCopy()
	existing.Status = desired.Status

	trig, err := r.EventingClientSet.EventingV1alpha1().Triggers(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(trig.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("Trigger %q became ready after %v", trigger.Name, duration)
		r.Recorder.Event(trigger, corev1.EventTypeNormal, triggerReadinessChanged, fmt.Sprintf("Trigger %q became ready", trigger.Name))
		if err := r.StatsReporter.ReportReady("Trigger", trigger.Namespace, trigger.Name, duration); err != nil {
			logging.FromContext(ctx).Sugar().Infof("failed to record ready for Trigger, %v", err)
		}
	}

	return trig, err
}

func (r *Reconciler) checkDependencyAnnotation(ctx context.Context, t *v1alpha1.Trigger, b *v1alpha1.Broker) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[v1alpha1.DependencyAnnotation]; ok {
		dependencyObjRef, err := v1alpha1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		trackKResource := r.kresourceTracker.TrackInNamespace(b)
		// Trigger and its dependent source are in the same namespace, we already did the validation in the webhook.
		if err := trackKResource(dependencyObjRef); err != nil {
			return fmt.Errorf("tracking dependency: %v", err)
		}
		if err := r.propagateDependencyReadiness(ctx, t, dependencyObjRef); err != nil {
			return fmt.Errorf("propagating dependency readiness: %v", err)
		}
	} else {
		t.Status.MarkDependencySucceeded()
	}
	return nil
}

func (r *Reconciler) propagateDependencyReadiness(ctx context.Context, t *v1alpha1.Trigger, dependencyObjRef corev1.ObjectReference) error {
	lister, err := r.kresourceTracker.ListerFor(dependencyObjRef)
	if err != nil {
		t.Status.MarkDependencyUnknown("ListerDoesNotExist", "Failed to retrieve lister: %v", err)
		return fmt.Errorf("retrieving lister: %v", err)
	}
	dependencyObj, err := lister.ByNamespace(t.GetNamespace()).Get(dependencyObjRef.Name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			t.Status.MarkDependencyFailed("DependencyDoesNotExist", "Dependency does not exist: %v", err)
		} else {
			t.Status.MarkDependencyUnknown("DependencyGetFailed", "Failed to get dependency: %v", err)
		}
		return fmt.Errorf("getting the dependency: %v", err)
	}
	dependency := dependencyObj.(*duckv1.KResource)

	// The dependency hasn't yet reconciled our latest changes to
	// its desired state, so its conditions are outdated.
	if dependency.GetGeneration() != dependency.Status.ObservedGeneration {
		logging.FromContext(ctx).Info("The ObjectMeta Generation of dependency is not equal to the observedGeneration of status",
			zap.Any("objectMetaGeneration", dependency.GetGeneration()),
			zap.Any("statusObservedGeneration", dependency.Status.ObservedGeneration))
		t.Status.MarkDependencyUnknown("GenerationNotEqual", "The dependency's metadata.generation, %q, is not equal to its status.observedGeneration, %q.", dependency.GetGeneration(), dependency.Status.ObservedGeneration)
		return nil
	}
	t.Status.PropagateDependencyStatus(dependency)
	return nil
}
