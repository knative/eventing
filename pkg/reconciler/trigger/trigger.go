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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	messaginglisters "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/duck"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	brokerresources "knative.dev/eventing/pkg/reconciler/broker/resources"
	"knative.dev/eventing/pkg/reconciler/names"
	"knative.dev/eventing/pkg/reconciler/trigger/path"
	"knative.dev/eventing/pkg/reconciler/trigger/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
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

type Reconciler struct {
	*reconciler.Base

	triggerLister      listers.TriggerLister
	subscriptionLister messaginglisters.SubscriptionLister
	brokerLister       listers.BrokerLister
	serviceLister      corev1listers.ServiceLister
	namespaceLister    corev1listers.NamespaceLister
	// Regular tracker to track static resources. In particular, it tracks Broker's changes.
	tracker tracker.Interface
	// Dynamic tracker to track KResources. In particular, it tracks the dependency between Triggers and Sources.
	kresourceTracker duck.ListableTracker
	// Dynamic tracker to track AddressableTypes. In particular, it tracks Trigger subscribers.
	addressableTracker duck.ListableTracker
	uriResolver        *resolver.URIResolver
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
		r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", updateStatusErr)
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
	// 5. Find whether there is annotation with key "knative.dev/dependency".
	// If not, mark Dependency to be succeeded, else figure out whether the dependency is ready and mark Dependency correspondingly

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}
	// Tell tracker to reconcile this Trigger whenever the Broker changes.
	brokerObjRef := corev1.ObjectReference{
		Kind:       brokerGVK.Kind,
		APIVersion: brokerGVK.GroupVersion().String(),
		Name:       t.Spec.Broker,
		Namespace:  t.Namespace,
	}

	if err := r.tracker.Track(brokerObjRef, t); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	b, err := r.brokerLister.Brokers(t.Namespace).Get(t.Spec.Broker)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		if apierrs.IsNotFound(err) {
			t.Status.MarkBrokerFailed("DoesNotExist", "Broker does not exist")
			_, needDefaultBroker := t.GetAnnotations()[v1alpha1.InjectionAnnotation]
			if t.Spec.Broker == "default" && needDefaultBroker {
				if e := r.labelNamespace(ctx, t); e != nil {
					logging.FromContext(ctx).Error("Unable to label the namespace", zap.Error(e))
				}
			}
		} else {
			t.Status.MarkBrokerFailed("BrokerGetFailed", "Failed to get broker")
		}
		return err
	}
	t.Status.PropagateBrokerStatus(&b.Status)

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

	subscriberURI, err := r.uriResolver.URIFromDestinationV1(t.Spec.Subscriber, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		t.Status.MarkSubscriberResolvedFailed("Unable to get the Subscriber's URI", "%v", err)
		t.Status.SubscriberURI = nil
		return err
	}
	t.Status.SubscriberURI = subscriberURI
	t.Status.MarkSubscriberResolvedSucceeded()

	sub, err := r.subscribeToBrokerChannel(ctx, t, brokerTrigger, &brokerObjRef, filterSvc)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("NotSubscribed", "%v", err)
		return err
	}
	t.Status.PropagateSubscriptionStatus(&sub.Status)

	if err := r.checkDependencyAnnotation(ctx, t); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) checkDependencyAnnotation(ctx context.Context, t *v1alpha1.Trigger) error {
	if dependencyAnnotation, ok := t.GetAnnotations()[v1alpha1.DependencyAnnotation]; ok {
		dependencyObjRef, err := v1alpha1.GetObjRefFromDependencyAnnotation(dependencyAnnotation)
		if err != nil {
			t.Status.MarkDependencyFailed("ReferenceError", "Unable to unmarshal objectReference from dependency annotation of trigger: %v", err)
			return fmt.Errorf("getting object ref from dependency annotation %q: %v", dependencyAnnotation, err)
		}
		trackKResource := r.kresourceTracker.TrackInNamespace(t)
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
			t.Status.MarkDependencyUnknown("DependencyDoesNotExist", "Dependency does not exist: %v", err)
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

// labelNamespace will label namespace with knative-eventing-injection=enabled
func (r *Reconciler) labelNamespace(ctx context.Context, t *v1alpha1.Trigger) error {
	current, err := r.namespaceLister.Get(t.Namespace)
	if err != nil {
		t.Status.MarkBrokerFailed("NamespaceGetFailed", "Failed to get namespace resource to enable knative-eventing-injection")
		return err
	}
	current = current.DeepCopy()
	if current.Labels == nil {
		current.Labels = map[string]string{}
	}
	current.Labels["knative-eventing-injection"] = "enabled"
	if _, err = r.KubeClientSet.CoreV1().Namespaces().Update(current); err != nil {
		t.Status.MarkBrokerFailed("NamespaceUpdateFailed", "Failed to label the namespace resource with knative-eventing-injection")
		return err
	}
	return nil
}

// getBrokerFilterService returns the K8s service for trigger 't' if exists,
// otherwise it returns an error.
func (r *Reconciler) getBrokerFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	svcName := brokerresources.MakeFilterService(b).Name
	return r.serviceLister.Services(b.Namespace).Get(svcName)
}

// subscribeToBrokerChannel subscribes service 'svc' to the Broker's channels.
func (r *Reconciler) subscribeToBrokerChannel(ctx context.Context, t *v1alpha1.Trigger, brokerTrigger, brokerRef *corev1.ObjectReference, svc *corev1.Service) (*messagingv1alpha1.Subscription, error) {
	uri := &url.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
		Path:   path.Generate(t),
	}
	expected := resources.NewSubscription(t, brokerTrigger, brokerRef, uri)

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
