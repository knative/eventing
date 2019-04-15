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
	"net/url"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	brokerresources "github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/trigger/path"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/trigger/resources"
	"github.com/knative/eventing/pkg/utils/resolve"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "trigger-controller"

	// Name of the corev1.Events emitted from the reconciliation process.
	triggerReconciled         = "TriggerReconciled"
	triggerReconcileFailed    = "TriggerReconcileFailed"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
	subscriptionDeleteFailed  = "SubscriptionDeleteFailed"
	subscriptionCreateFailed  = "SubscriptionCreateFailed"
)

type reconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler.
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a function that returns a Trigger controller.
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	// Setup a new controller to Reconcile Triggers.
	r := &reconciler{
		recorder: mgr.GetRecorder(controllerAgentName),
		logger:   logger,
	}
	c, err := controller.New(controllerAgentName, mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return nil, err
	}

	// Watch Triggers.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Trigger{}}, &handler.EnqueueRequestForObject{}); err != nil {
		return nil, err
	}

	// Watch all the resources that the Trigger reconciles.
	for _, t := range []runtime.Object{&corev1.Service{}, &istiov1alpha3.VirtualService{}, &v1alpha1.Subscription{}} {
		err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Trigger{}, IsController: true})
		if err != nil {
			return nil, err
		}
	}

	// Watch for Broker changes. E.g. if the Broker is deleted and recreated, we need to reconcile
	// the Trigger again.
	if err = c.Watch(&source.Kind{Type: &v1alpha1.Broker{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &mapBrokerToTriggers{r: r}}); err != nil {
		return nil, err
	}

	// TODO reconcile after a change to the subscriber. I'm not sure how this is possible, but we should do it if we
	// can find a way.

	return c, nil
}

// mapBrokerToTriggers maps Broker changes to all the Triggers that correspond to that Broker.
type mapBrokerToTriggers struct {
	r *reconciler
}

// Map implements handler.Mapper.Map.
func (b *mapBrokerToTriggers) Map(o handler.MapObject) []reconcile.Request {
	ctx := context.Background()
	triggers := make([]reconcile.Request, 0)

	opts := &client.ListOptions{
		Namespace: o.Meta.GetNamespace(),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		tl := &v1alpha1.TriggerList{}
		if err := b.r.client.List(ctx, opts, tl); err != nil {
			b.r.logger.Error("Error listing Triggers when Broker changed. Some Triggers may not be reconciled.", zap.Error(err), zap.Any("broker", o))
			return triggers
		}

		for _, t := range tl.Items {
			if t.Spec.Broker == o.Meta.GetName() {
				triggers = append(triggers, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Namespace: t.Namespace,
						Name:      t.Name,
					},
				})
			}
		}
		if tl.Continue != "" {
			opts.Raw.Continue = tl.Continue
		} else {
			return triggers
		}
	}
}

// InjectClient implements controller runtime's inject.Client.
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// InjectConfig implements controller runtime's inject.Config.
func (r *reconciler) InjectConfig(c *rest.Config) error {
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	return err
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Trigger resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	trigger := &v1alpha1.Trigger{}
	err := r.client.Get(ctx, request.NamespacedName, trigger)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Trigger")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not get Trigger", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Reconcile this copy of the Trigger and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, trigger)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Trigger", zap.Error(reconcileErr))
		r.recorder.Eventf(trigger, corev1.EventTypeWarning, triggerReconcileFailed, "Trigger reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("Trigger reconciled")
		r.recorder.Event(trigger, corev1.EventTypeNormal, triggerReconciled, "Trigger reconciled")
	}

	if _, err = r.updateStatus(trigger); err != nil {
		logging.FromContext(ctx).Error("Failed to update Trigger status", zap.Error(err))
		r.recorder.Eventf(trigger, corev1.EventTypeWarning, triggerUpdateStatusFailed, "Failed to update Trigger's status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready
	return reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, t *v1alpha1.Trigger) error {
	t.Status.InitializeConditions()

	// 1. Verify the Broker exists.
	// 2. Get the Broker's:
	//   - Filter Channel
	//   - Ingress Channel
	//   - Filter Service
	// 3. Find the Subscriber's URI.
	// 4. Creates a Subscription from the Broker's Filter Channel to this Trigger via the Broker's
	//    Filter Service with a specific path, and reply set to the Broker's Ingress Channel.

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	b, err := r.getBroker(ctx, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		t.Status.MarkBrokerDoesNotExist()
		return err
	}
	t.Status.MarkBrokerExists()

	brokerTrigger, err := r.getBrokerTriggerChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker's Trigger Channel", zap.Error(err))
		return err
	}
	brokerIngress, err := r.getBrokerIngressChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker's Ingress Channel", zap.Error(err))
		return err
	}
	// Get Broker filter service.
	filterSvc, err := r.getBrokerFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker's filter Service", zap.Error(err))
		return err
	}

	subscriberURI, err := resolve.SubscriberSpec(ctx, r.dynamicClient, t.Namespace, t.Spec.Subscriber)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		return err
	}
	t.Status.SubscriberURI = subscriberURI

	_, err = r.subscribeToBrokerChannel(ctx, t, brokerTrigger, brokerIngress, filterSvc)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Subscribe", zap.Error(err))
		t.Status.MarkNotSubscribed("notSubscribed", "%v", err)
		return err
	}
	t.Status.MarkSubscribed()

	return nil
}

// updateStatus may in fact update the trigger's finalizers in addition to the status.
func (r *reconciler) updateStatus(trigger *v1alpha1.Trigger) (*v1alpha1.Trigger, error) {
	ctx := context.TODO()
	objectKey := client.ObjectKey{Namespace: trigger.Namespace, Name: trigger.Name}
	latestTrigger := &v1alpha1.Trigger{}

	if err := r.client.Get(ctx, objectKey, latestTrigger); err != nil {
		return nil, err
	}

	triggerChanged := false

	if !equality.Semantic.DeepEqual(latestTrigger.Finalizers, trigger.Finalizers) {
		latestTrigger.SetFinalizers(trigger.ObjectMeta.Finalizers)
		if err := r.client.Update(ctx, latestTrigger); err != nil {
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
		if err := r.client.Get(ctx, objectKey, latestTrigger); err != nil {
			return nil, err
		}
	}

	latestTrigger.Status = trigger.Status
	if err := r.client.Status().Update(ctx, latestTrigger); err != nil {
		return nil, err
	}

	return latestTrigger, nil
}

// getBroker returns the Broker for Trigger 't' if exists, otherwise it returns an error.
func (r *reconciler) getBroker(ctx context.Context, t *v1alpha1.Trigger) (*v1alpha1.Broker, error) {
	b := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Spec.Broker,
	}
	err := r.client.Get(ctx, name, b)
	return b, err
}

// getBrokerTriggerChannel return the Broker's Trigger Channel if it exists, otherwise it returns an
// error.
func (r *reconciler) getBrokerTriggerChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	return r.getChannel(ctx, b, labels.SelectorFromSet(broker.TriggerChannelLabels(b)))
}

// getBrokerIngressChannel return the Broker's Ingress Channel if it exists, otherwise it returns an
// error.
func (r *reconciler) getBrokerIngressChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	return r.getChannel(ctx, b, labels.SelectorFromSet(broker.IngressChannelLabels(b)))
}

// getChannel returns the Broker's channel if it exists, otherwise it returns an error.
func (r *reconciler) getChannel(ctx context.Context, b *v1alpha1.Broker, ls labels.Selector) (*v1alpha1.Channel, error) {
	list := &v1alpha1.ChannelList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: ls,
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
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

// getService returns the K8s service for trigger 't' if exists,
// otherwise it returns an error.
func (r *reconciler) getBrokerFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	list := &corev1.ServiceList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: labels.SelectorFromSet(brokerresources.FilterLabels(b)),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}

	err := r.client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, svc := range list.Items {
		if metav1.IsControlledBy(&svc, b) {
			return &svc, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

// subscribeToBrokerChannel subscribes service 'svc' to the Broker's channels.
func (r *reconciler) subscribeToBrokerChannel(ctx context.Context, t *v1alpha1.Trigger, brokerTrigger, brokerIngress *v1alpha1.Channel, svc *corev1.Service) (*v1alpha1.Subscription, error) {
	uri := &url.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
		Path:   path.Generate(t),
	}
	expected := resources.NewSubscription(t, brokerTrigger, brokerIngress, uri)

	sub, err := r.getSubscription(ctx, t)
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

	// Update Subscription if it has changed. Ignore the generation.
	expected.Spec.DeprecatedGeneration = sub.Spec.DeprecatedGeneration
	if !equality.Semantic.DeepDerivative(expected.Spec, sub.Spec) {
		// Given that spec.channel is immutable, we cannot just update the Subscription. We delete
		// it and re-create it instead.
		err = r.client.Delete(ctx, sub)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot delete subscription", zap.Error(err))
			r.recorder.Eventf(t, corev1.EventTypeWarning, subscriptionDeleteFailed, "Delete Trigger's subscription failed: %v", err)
			return nil, err
		}
		sub = expected
		err = r.client.Create(ctx, sub)
		if err != nil {
			logging.FromContext(ctx).Info("Cannot create subscription", zap.Error(err))
			r.recorder.Eventf(t, corev1.EventTypeWarning, subscriptionCreateFailed, "Create Trigger's subscription failed: %v", err)
			return nil, err
		}
	}
	return sub, nil
}

// getSubscription returns the subscription of trigger 't' if exists,
// otherwise it returns an error.
func (r *reconciler) getSubscription(ctx context.Context, t *v1alpha1.Trigger) (*v1alpha1.Subscription, error) {
	list := &v1alpha1.SubscriptionList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     t.Namespace,
		LabelSelector: labels.SelectorFromSet(resources.SubscriptionLabels(t)),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
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
