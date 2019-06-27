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

package trigger

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/duck"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	brokerresources "github.com/knative/eventing/pkg/reconciler/broker/resources"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/reconciler/trigger/path"
	"github.com/knative/eventing/pkg/reconciler/trigger/resources"
	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
)

const (
	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	triggerReconciled         = "TriggerReconciled"
	triggerReadinessChanged   = "TriggerReadinessChanged"
	triggerReconcileFailed    = "TriggerReconcileFailed"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
	subscriptionDeleteFailed  = "SubscriptionDeleteFailed"
	subscriptionCreateFailed  = "SubscriptionCreateFailed"
	triggerChannelFailed      = "TriggerChannelFailed"
	ingressChannelFailed      = "IngressChannelFailed"
	triggerServiceFailed      = "TriggerServiceFailed"
)

type Reconciler struct {
	*reconciler.Base

	triggerLister      listers.TriggerLister
	channelLister      listers.ChannelLister
	subscriptionLister listers.SubscriptionLister
	brokerLister       listers.BrokerLister
	serviceLister      corev1listers.ServiceLister
	addressableTracker duck.AddressableTracker
}

var brokerGVK = v1alpha1.SchemeGroupVersion.WithKind("Broker")

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the Trigger resource with this namespace/name.
	original, err := r.triggerLister.Triggers(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("trigger key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	trigger := original.DeepCopy()

	// Reconcile this copy of the Trigger and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, trigger)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Trigger", zap.Error(reconcileErr))
		r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Trigger reconciled")
		r.Recorder.Event(trigger, corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled")
	}

	if _, updateStatusErr := r.updateStatus(ctx, trigger); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update Trigger status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, t *v1alpha1.Trigger) error {
	t.Status.InitializeConditions()

	// 1. Verify the Broker exists.
	// 2. Get the Broker's:
	//   - Trigger Channel
	//   - Ingress Channel
	//   - Filter Service
	// 3. Find the Subscriber's URI.
	// 4. Creates a Subscription from the Broker's Trigger Channel to this Trigger via the Broker's
	//    Filter Service with a specific path, and reply set to the Broker's Ingress Channel.

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		if apierrs.IsNotFound(err) {
			t.Status.MarkBrokerFailed("DoesNotExist", "Broker does not exist")
		} else {
			t.Status.MarkBrokerFailed("BrokerGetFailed", "Failed to get broker")
		}
		return err
	}
	t.Status.PropagateBrokerStatus(&b.Status)

	// Tell addressableTracker to reconcile this Trigger whenever the Broker changes.
	track := r.addressableTracker.TrackInNamespace(t)
	if err = track(utils.ObjectRef(b, brokerGVK)); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	brokerTrigger := b.Status.TriggerChannel
	if brokerTrigger == nil {
		logging.FromContext(ctx).Error("Broker TriggerChannel not populated")
		r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerChannelFailed, "Broker's Trigger channel not found")
		return errors.New("failed to find Broker's Trigger channel")
	}

	brokerIngress := b.Status.IngressChannel
	if brokerIngress == nil {
		logging.FromContext(ctx).Error("Broker IngressrChannel not populated")
		r.Recorder.Eventf(t, corev1.EventTypeWarning, ingressChannelFailed, "Broker's Ingress channel not found")
		return errors.New("failed to find Broker's Ingress channel")
	}

	// Get Broker filter service.
	filterSvc, err := r.getBrokerFilterService(ctx, b)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Error("can not find Broker's Filter service", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerServiceFailed, "Broker's Filter service not found")
			return errors.New("failed to find Broker's Filter service")
		} else {
			logging.FromContext(ctx).Error("failed to get Broker's Filter service", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerServiceFailed, "Failed to get Broker's Filter service")
			return err
		}
	}

	subscriberURI, err := duck.SubscriberSpec(ctx, r.DynamicClientSet, t.Namespace, t.Spec.Subscriber, track)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		return err
	}
	t.Status.SubscriberURI = subscriberURI

	sub, err := r.subscribeToBrokerChannel(ctx, t, brokerTrigger, brokerIngress, filterSvc)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("NotSubscribed", "%v", err)
		return err
	}
	t.Status.PropagateSubscriptionStatus(&sub.Status)

	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.Trigger) (*v1alpha1.Trigger, error) {
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

// getBrokerFilterService returns the K8s service for trigger 't' if exists,
// otherwise it returns an error.
func (r *Reconciler) getBrokerFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	services, err := r.serviceLister.Services(b.Namespace).List(labels.SelectorFromSet(brokerresources.FilterLabels(b.Name)))
	if err != nil {
		return nil, err
	}
	for _, svc := range services {
		if metav1.IsControlledBy(svc, b) {
			return svc, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}

// subscribeToBrokerChannel subscribes service 'svc' to the Broker's channels.
func (r *Reconciler) subscribeToBrokerChannel(ctx context.Context, t *v1alpha1.Trigger, brokerTrigger, brokerIngress *corev1.ObjectReference, svc *corev1.Service) (*v1alpha1.Subscription, error) {
	uri := &url.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
		Path:   path.Generate(t),
	}
	expected := resources.NewSubscription(t, brokerTrigger, brokerIngress, uri)

	sub, err := r.getSubscription(ctx, t)

	// If the resource doesn't exist, we'll create it.
	if apierrs.IsNotFound(err) {
		sub = expected
		logging.FromContext(ctx).Info("Creating subscription")
		newSub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
			return nil, err
		}
		return newSub, nil
	} else if err != nil {
		logging.FromContext(ctx).Error("Failed to get subscription", zap.Error(err))
		r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
		return nil, err
	}

	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = sub.Spec.DeprecatedGeneration
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the Subscription. We delete
		// it and re-create it instead.
		logging.FromContext(ctx).Info("Deleting subscription", zap.String("namespace", sub.Namespace), zap.String("name", sub.Name))
		err = r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionDeleteFailed, "Delete Trigger's subscription failed: %v", err)
			return nil, err
		}
		sub = expected
		logging.FromContext(ctx).Info("Creating subscription")
		newSub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
			return nil, err
		}
		return newSub, nil
	}
	return sub, nil
}

// getSubscription returns the subscription of trigger 't' if exists,
// otherwise it returns an error.
func (r *Reconciler) getSubscription(ctx context.Context, t *v1alpha1.Trigger) (*v1alpha1.Subscription, error) {
	subs, err := r.subscriptionLister.Subscriptions(t.Namespace).List(labels.SelectorFromSet(resources.SubscriptionLabels(t)))
	if err != nil {
		return nil, err
	}
	for _, s := range subs {
		if metav1.IsControlledBy(s, t) {
			return s, nil
		}
	}

	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}
