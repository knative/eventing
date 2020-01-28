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

package broker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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
)

func (r *Reconciler) reconcileTrigger(ctx context.Context, b *v1alpha1.Broker, t *v1alpha1.Trigger) error {
	t.Status.InitializeConditions()

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	brokerTrigger := b.Status.TriggerChannel
	if brokerTrigger == nil {
		logging.FromContext(ctx).Error("Broker TriggerChannel not populated")
		r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerChannelFailed, "Broker's Trigger channel not found")
		return errors.New("failed to find Broker's Trigger channel")
	}

	// Get Broker filter service.
	filterSvc, err := r.getBrokerFilterService(ctx, b)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Error("can not find Broker's Filter service", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerServiceFailed, "Broker's Filter service not found")
			return errors.New("failed to find Broker's Filter service")
		}
		logging.FromContext(ctx).Error("failed to get Broker's Filter service", zap.Error(err))
		r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerServiceFailed, "Failed to get Broker's Filter service")
		return err
	}

	if t.Spec.Subscriber.Ref != nil {
		// To call URIFromDestination(dest apisv1alpha1.Destination, parent interface{}), dest.Ref must have a Namespace
		// We will use the Namespace of Trigger as the Namespace of dest.Ref
		t.Spec.Subscriber.Ref.Namespace = t.GetNamespace()
		// Since Trigger never allowed ref fields at the subscriber level and
		// validates that they are absent, we can ignore them here.
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

	sub, err := r.subscribeToBrokerChannel(ctx, b, t, brokerTrigger, filterSvc)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("NotSubscribed", "%v", err)
		return err
	}
	t.Status.PropagateSubscriptionStatus(&sub.Status)

	// DO NOT SUBMIT
	// TODO...
	/*
		if err := r.checkDependencyAnnotation(ctx, t); err != nil {
			return err
		}
	*/

	return nil
}

// getBrokerFilterService returns the K8s service for trigger 't' if exists,
// otherwise it returns an error.
func (r *Reconciler) getBrokerFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	return r.serviceLister.Services(b.Namespace).Get(fmt.Sprintf("%s-broker-filter", b.Name))
}

// subscribeToBrokerChannel subscribes service 'svc' to the Broker's channels.
func (r *Reconciler) subscribeToBrokerChannel(ctx context.Context, b *v1alpha1.Broker, t *v1alpha1.Trigger, brokerTrigger *corev1.ObjectReference, svc *corev1.Service) (*messagingv1alpha1.Subscription, error) {
	uri := &url.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
		Path:   path.Generate(t),
	}
	logging.FromContext(ctx).Info("VAIKAS BROKER METADATA", zap.String("APIVERSION", b.TypeMeta.APIVersion), zap.String("KIND", b.TypeMeta.Kind))
	brokerObjRef := &corev1.ObjectReference{
		//		Kind:       b.TypeMeta.Kind,
		//		APIVersion: b.TypeMeta.APIVersion,
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
		// TODO Add logging
		return nil, err
	}
	return sub, nil
}

func (r *Reconciler) reconcileSubscription(ctx context.Context, t *v1alpha1.Trigger, expected, actual *messagingv1alpha1.Subscription) (*messagingv1alpha1.Subscription, error) {
	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = actual.Spec.DeprecatedGeneration
	if equality.Semantic.DeepDerivative(expected.Spec, actual.Spec) {
		return actual, nil
	}

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
