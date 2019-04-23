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
	"net/url"
	"reflect"
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventinginformers "github.com/knative/eventing/pkg/client/informers/externalversions/eventing/v1alpha1"
	listers "github.com/knative/eventing/pkg/client/listers/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/reconciler/trigger/path"
	"github.com/knative/eventing/pkg/reconciler/trigger/resources"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	brokerresources "github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"github.com/knative/eventing/pkg/utils/resolve"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/tracker"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	// ReconcilerName is the name of the reconciler
	ReconcilerName = "Triggers"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "trigger-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process.
	triggerReconciled         = "TriggerReconciled"
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
	tracker            tracker.Interface
}

var brokerGVK = v1alpha1.SchemeGroupVersion.WithKind("Broker")

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	triggerInformer eventinginformers.TriggerInformer,
	channelInformer eventinginformers.ChannelInformer,
	subscriptionInformer eventinginformers.SubscriptionInformer,
	brokerInformer eventinginformers.BrokerInformer,
	serviceInformer corev1informers.ServiceInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:               reconciler.NewBase(opt, controllerAgentName),
		triggerLister:      triggerInformer.Lister(),
		channelLister:      channelInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		brokerLister:       brokerInformer.Lister(),
		serviceLister:      serviceInformer.Lister(),
	}
	impl := controller.NewImpl(r, r.Logger, ReconcilerName, reconciler.MustNewStatsReporter(ReconcilerName, r.Logger))

	r.Logger.Info("Setting up event handlers")
	triggerInformer.Informer().AddEventHandler(reconciler.Handler(impl.Enqueue))

	// Tracker is used to notify us that a Trigger's Broker has changed so that
	// we can reconcile.
	r.tracker = tracker.New(impl.EnqueueKey, opt.GetTrackerLease())
	brokerInformer.Informer().AddEventHandler(reconciler.Handler(
		// Call the tracker's OnChanged method, but we've seen the objects
		// coming through this path missing TypeMeta, so ensure it is properly
		// populated.
		controller.EnsureTypeMeta(
			r.tracker.OnChanged,
			v1alpha1.SchemeGroupVersion.WithKind("Broker"),
		),
	))

	subscriptionInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("Trigger")),
		Handler:    reconciler.Handler(impl.EnqueueControllerOf),
	})
	return impl
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	ctx = logging.WithLogger(ctx, r.Logger.Desugar().With(zap.String("key", key)))

	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
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
	err = r.reconcile(ctx, trigger)
	if err != nil {
		logging.FromContext(ctx).Error("Error reconciling Trigger", zap.Error(err))
		r.Recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconciliation failed: %v", err)
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
	return err
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

	// Tell tracker to reconcile this Trigger whenever the Broker changes.
	if err = r.tracker.Track(objectRef(b, brokerGVK), t); err != nil {
		logging.FromContext(ctx).Error("Unable to track changes to Broker", zap.Error(err))
		return err
	}

	brokerTrigger, err := r.getBrokerTriggerChannel(ctx, b)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Error("can not find Broker's Trigger Channel", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerChannelFailed, "Broker's Trigger channel not found")
			return errors.New("failed to find Broker's Trigger channel")
		} else {
			logging.FromContext(ctx).Error("failed to get Broker's Trigger Channel", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, triggerChannelFailed, "Failed to get Broker's Trigger channel")
			return err
		}
	}
	brokerIngress, err := r.getBrokerIngressChannel(ctx, b)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Error("can not find Broker's Ingress Channel", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, ingressChannelFailed, "Broker's Ingress channel not found")
			return errors.New("failed to find Broker's Ingress channel")
		} else {
			logging.FromContext(ctx).Error("failed to get Broker's Ingress Channel", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, ingressChannelFailed, "Failed to get Broker's Ingress channel")
			return err
		}
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

	subscriberURI, err := resolve.SubscriberSpec(ctx, r.DynamicClientSet, t.Namespace, t.Spec.Subscriber)
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

	new, err := r.EventingClientSet.EventingV1alpha1().Triggers(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(new.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("Subscription %q became ready after %v", trigger.Name, duration)
		//r.StatsReporter.ReportServiceReady(trigger.Namespace, trigger.Name, duration) // TODO: stats
	}

	return new, err
}

// getBrokerTriggerChannel return the Broker's Trigger Channel if it exists, otherwise it returns an
// error.
func (r *Reconciler) getBrokerTriggerChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	return r.getChannel(ctx, b, labels.SelectorFromSet(broker.TriggerChannelLabels(b)))
}

// getBrokerIngressChannel return the Broker's Ingress Channel if it exists, otherwise it returns an
// error.
func (r *Reconciler) getBrokerIngressChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	return r.getChannel(ctx, b, labels.SelectorFromSet(broker.IngressChannelLabels(b)))
}

// getChannel returns the Broker's channel based on the provided label selector if it exists, otherwise it returns an error.
func (r *Reconciler) getChannel(ctx context.Context, b *v1alpha1.Broker, ls labels.Selector) (*v1alpha1.Channel, error) {
	channels, err := r.channelLister.Channels(b.Namespace).List(ls)
	if err != nil {
		return nil, err
	}

	// TODO: If there's more than one, should that be treated as an error. This seems a bit wonky
	// but that's how it was before.
	for _, c := range channels {
		if metav1.IsControlledBy(c, b) {
			return c, nil
		}
	}
	return nil, apierrs.NewNotFound(schema.GroupResource{}, "")
}

// getBrokerFilterService returns the K8s service for trigger 't' if exists,
// otherwise it returns an error.
func (r *Reconciler) getBrokerFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	services, err := r.serviceLister.Services(b.Namespace).List(labels.SelectorFromSet(brokerresources.FilterLabels(b)))
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
func (r *Reconciler) subscribeToBrokerChannel(ctx context.Context, t *v1alpha1.Trigger, brokerTrigger, brokerIngress *v1alpha1.Channel, svc *corev1.Service) (*v1alpha1.Subscription, error) {
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
		newSub, err := r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Create(sub)
		if err != nil {
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
			return nil, err
		}
		return newSub, nil
	} else if err != nil {
		r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
		return nil, err
	}

	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = sub.Spec.DeprecatedGeneration
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the Subscription. We delete
		// it and re-create it instead.
		err = r.EventingClientSet.EventingV1alpha1().Subscriptions(sub.Namespace).Delete(sub.Name, &metav1.DeleteOptions{})
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			r.Recorder.Eventf(t, corev1.EventTypeWarning, subscriptionDeleteFailed, "Delete Trigger's subscription failed: %v", err)
			return nil, err
		}
		sub = expected
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

type accessor interface {
	GroupVersionKind() schema.GroupVersionKind
	GetNamespace() string
	GetName() string
}

func objectRef(a accessor, gvk schema.GroupVersionKind) corev1.ObjectReference {
	// We can't always rely on the TypeMeta being populated.
	// See: https://github.com/knative/serving/issues/2372
	// Also: https://github.com/kubernetes/apiextensions-apiserver/issues/29
	// gvk := a.GroupVersionKind()
	apiVersion, kind := gvk.ToAPIVersionAndKind()
	return corev1.ObjectReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  a.GetNamespace(),
		Name:       a.GetName(),
	}
}
