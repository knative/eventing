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
	"sync"

	"github.com/knative/eventing/pkg/provisioners"

	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/knative/eventing/pkg/reconciler/names"

	"github.com/knative/eventing/contrib/gcppubsub/pkg/util/logging"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/subscription"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "trigger-controller"

	finalizerName = controllerAgentName

	// Name of the corev1.Events emitted from the reconciliation process
	triggerReconciled         = "TriggerReconciled"
	triggerReconcileFailed    = "TriggerReconcileFailed"
	triggerUpdateStatusFailed = "TriggerUpdateStatusFailed"
	subscriptionDeleteFailed  = "SubscriptionDeleteFailed"
	subscriptionCreateFailed  = "SubscriptionCreateFailed"
)

var dummyValue struct{}

type reconciler struct {
	client        client.Client
	restConfig    *rest.Config
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder

	triggersLock sync.RWMutex
	// Contains the triggers that correspond to a particular broker.
	// We use this to reconcile only the triggers that correspond to a certain broker.
	// brokerNamespacedName -> triggerReconcileRequest -> dummy struct
	triggers map[types.NamespacedName]map[reconcile.Request]struct{}

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

// ProvideController returns a function that returns a Trigger controller.
func ProvideController(logger *zap.Logger) func(manager.Manager) (controller.Controller, error) {
	return func(mgr manager.Manager) (controller.Controller, error) {
		// Setup a new controller to Reconcile Triggers.
		r := &reconciler{
			recorder: mgr.GetRecorder(controllerAgentName),
			logger:   logger,
			triggers: make(map[types.NamespacedName]map[reconcile.Request]struct{}),
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

		// Watch for Broker changes.
		if err = c.Watch(&source.Kind{Type: &v1alpha1.Broker{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &mapAllTriggers{r}}); err != nil {
			return nil, err
		}

		// TODO reconcile after a change to the subscriber. I'm not sure how this is possible, but we should do it if we
		// can find a way.

		return c, nil
	}
}

type mapAllTriggers struct {
	r *reconciler
}

func (m *mapAllTriggers) Map(o handler.MapObject) []reconcile.Request {
	m.r.triggersLock.RLock()
	defer m.r.triggersLock.RUnlock()
	brokerNamespacedName := types.NamespacedName{Namespace: o.Meta.GetNamespace(), Name: o.Meta.GetName()}
	triggersInBrokerNamespacedName := m.r.triggers[brokerNamespacedName]
	if triggersInBrokerNamespacedName == nil {
		return []reconcile.Request{}
	}
	reqs := make([]reconcile.Request, 0, len(triggersInBrokerNamespacedName))
	for name := range triggersInBrokerNamespacedName {
		reqs = append(reqs, name)
	}
	return reqs
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) InjectConfig(c *rest.Config) error {
	r.restConfig = c
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
	err := r.client.Get(context.TODO(), request.NamespacedName, trigger)

	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find Trigger")
		return reconcile.Result{}, nil
	}

	if err != nil {
		logging.FromContext(ctx).Error("Could not get Trigger", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Modify a copy, not the original.
	trigger = trigger.DeepCopy()

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
	// 2. Find the Subscriber's URI.
	// 2. Creates a K8s Service uniquely named for this Trigger.
	// 3. Creates a VirtualService that routes the K8s Service to the Broker's filter service on an identifiable host name.
	// 4. Creates a Subscription from the Broker's single Channel to this Trigger's K8s Service, with reply set to the Broker.

	if t.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		r.removeFromTriggers(t)
		provisioners.RemoveFinalizer(t, finalizerName)
		return nil
	}

	provisioners.AddFinalizer(t, finalizerName)
	r.AddToTriggers(t)

	b, err := r.getBroker(ctx, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker", zap.Error(err))
		t.Status.MarkBrokerDoesNotExists()
		return err
	}
	t.Status.MarkBrokerExists()

	c, err := r.getBrokerChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Broker's Channel", zap.Error(err))
		return err
	}

	subscriberURI, err := subscription.ResolveSubscriberSpec(ctx, r.client, r.dynamicClient, t.Namespace, t.Spec.Subscriber)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to get the Subscriber's URI", zap.Error(err))
		return err
	}
	t.Status.SubscriberURI = subscriberURI

	svc, err := r.reconcileK8sService(ctx, t)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the K8s Service", zap.Error(err))
		return err
	}
	t.Status.MarkKubernetesServiceExists()

	_, err = r.reconcileVirtualService(ctx, t, svc)
	if err != nil {
		logging.FromContext(ctx).Error("Unable to reconcile the VirtualService", zap.Error(err))
		return err
	}
	t.Status.MarkVirtualServiceExists()

	_, err = r.subscribeToBrokerChannel(ctx, t, c, svc)
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

	brokerNamespacedName := types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Spec.Broker}

	// We will be reconciling an already existing Trigger far more often than adding a new one, so
	// check with a read lock before using the write lock.
	r.triggersLock.RLock()
	triggersInBrokerNamespacedName := r.triggers[brokerNamespacedName]
	var present bool
	if triggersInBrokerNamespacedName != nil {
		_, present = triggersInBrokerNamespacedName[name]
	} else {
		present = false
	}
	r.triggersLock.RUnlock()

	if present {
		// Already present in the map.
		return
	}

	r.triggersLock.Lock()
	triggersInBrokerNamespacedName = r.triggers[brokerNamespacedName]
	if triggersInBrokerNamespacedName == nil {
		r.triggers[brokerNamespacedName] = make(map[reconcile.Request]struct{})
		triggersInBrokerNamespacedName = r.triggers[brokerNamespacedName]
	}
	triggersInBrokerNamespacedName[name] = dummyValue
	r.triggersLock.Unlock()
}

func (r *reconciler) removeFromTriggers(t *v1alpha1.Trigger) {
	name := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: t.Namespace,
			Name:      t.Name,
		},
	}

	brokerNamespacedName := types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Spec.Broker}

	r.triggersLock.Lock()
	triggersInBrokerNamespacedName := r.triggers[brokerNamespacedName]
	if triggersInBrokerNamespacedName != nil {
		delete(triggersInBrokerNamespacedName, name)
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
	b := &v1alpha1.Broker{}
	name := types.NamespacedName{
		Namespace: t.Namespace,
		Name:      t.Spec.Broker,
	}
	err := r.client.Get(ctx, name, b)
	return b, err
}

func (r *reconciler) getBrokerChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	list := &v1alpha1.ChannelList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: labels.SelectorFromSet(broker.ChannelLabels(b)),
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

func (r *reconciler) getK8sService(ctx context.Context, t *v1alpha1.Trigger) (*corev1.Service, error) {
	list := &corev1.ServiceList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     t.Namespace,
		LabelSelector: labels.SelectorFromSet(k8sServiceLabels(t)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
		},
	}

	err := r.client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, svc := range list.Items {
		if metav1.IsControlledBy(&svc, t) {
			return &svc, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) reconcileK8sService(ctx context.Context, t *v1alpha1.Trigger) (*corev1.Service, error) {
	current, err := r.getK8sService(ctx, t)

	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		svc := newK8sService(t)
		err = r.client.Create(ctx, svc)
		if err != nil {
			return nil, err
		}
		return svc, nil
	} else if err != nil {
		return nil, err
	}

	expected := newK8sService(t)
	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	expected.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(expected.Spec, current.Spec) {
		current.Spec = expected.Spec
		err := r.client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

func newK8sService(t *v1alpha1.Trigger) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    t.Namespace,
			GenerateName: fmt.Sprintf("trigger-%s-", t.Name),
			Labels:       k8sServiceLabels(t),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Trigger",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
	}
}

func k8sServiceLabels(t *v1alpha1.Trigger) map[string]string {
	return map[string]string{
		"eventing.knative.dev/trigger": t.Name,
	}
}

func (r *reconciler) getVirtualService(ctx context.Context, t *v1alpha1.Trigger) (*istiov1alpha3.VirtualService, error) {
	list := &istiov1alpha3.VirtualServiceList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     t.Namespace,
		LabelSelector: labels.SelectorFromSet(virtualServiceLabels(t)),
		// TODO this is here because the fake client needs it. Remove this when it's no longer
		// needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istiov1alpha3.SchemeGroupVersion.String(),
				Kind:       "VirtualService",
			},
		},
	}

	err := r.client.List(ctx, opts, list)
	if err != nil {
		return nil, err
	}
	for _, vs := range list.Items {
		if metav1.IsControlledBy(&vs, t) {
			return &vs, nil
		}
	}

	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) reconcileVirtualService(ctx context.Context, t *v1alpha1.Trigger, svc *corev1.Service) (*istiov1alpha3.VirtualService, error) {
	virtualService, err := r.getVirtualService(ctx, t)

	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		virtualService = newVirtualService(t, svc)
		err = r.client.Create(ctx, virtualService)
		if err != nil {
			return nil, err
		}
		return virtualService, nil
	} else if err != nil {
		return nil, err
	}

	expected := newVirtualService(t, svc)
	if !equality.Semantic.DeepDerivative(expected.Spec, virtualService.Spec) {
		virtualService.Spec = expected.Spec
		err := r.client.Update(ctx, virtualService)
		if err != nil {
			return nil, err
		}
	}
	return virtualService, nil
}

func newVirtualService(t *v1alpha1.Trigger, svc *corev1.Service) *istiov1alpha3.VirtualService {
	// TODO Make this work with endings other than cluster.local
	destinationHost := fmt.Sprintf("%s-broker-filter.%s.svc.cluster.local", t.Spec.Broker, t.Namespace)
	return &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", t.Name),
			Namespace:    t.Namespace,
			Labels:       virtualServiceLabels(t),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Trigger",
				}),
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				names.ServiceHostName(svc.Name, svc.Namespace),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					// Never really used, so cluster.local should be a good enough ending everywhere.
					Authority: fmt.Sprintf("%s.%s.triggers.cluster.local", t.Name, t.Namespace),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: destinationHost,
						Port: istiov1alpha3.PortSelector{
							Number: 80,
						},
					}},
				}},
			},
		},
	}
}

func virtualServiceLabels(t *v1alpha1.Trigger) map[string]string {
	return map[string]string{
		"eventing.knative.dev/trigger": t.Name,
	}
}

func (r *reconciler) subscribeToBrokerChannel(ctx context.Context, t *v1alpha1.Trigger, c *v1alpha1.Channel, svc *corev1.Service) (*v1alpha1.Subscription, error) {
	expected := makeSubscription(t, c, svc)

	sub, err := r.getSubscription(ctx, t, c)
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
		// Given that the backing channel spec is immutable, we cannot just update the subscription.
		// We delete it instead, and re-create it.
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

func (r *reconciler) getSubscription(ctx context.Context, t *v1alpha1.Trigger, c *v1alpha1.Channel) (*v1alpha1.Subscription, error) {
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

func makeSubscription(t *v1alpha1.Trigger, c *v1alpha1.Channel, svc *corev1.Service) *v1alpha1.Subscription {
	return &v1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    t.Namespace,
			GenerateName: fmt.Sprintf("%s-%s-", t.Spec.Broker, t.Name),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(t, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Trigger",
				}),
			},
			Labels: subscriptionLabels(t),
		},
		Spec: v1alpha1.SubscriptionSpec{
			Channel: corev1.ObjectReference{
				APIVersion: v1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
				Name:       c.Name,
			},
			Subscriber: &v1alpha1.SubscriberSpec{
				Ref: &corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       svc.Name,
				},
			},
			// TODO This pushes directly into the Channel, it should probably point at the Broker ingress instead.
			Reply: &v1alpha1.ReplyStrategy{
				Channel: &corev1.ObjectReference{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       "Channel",
					Name:       c.Name,
				},
			},
		},
	}
}

func subscriptionLabels(t *v1alpha1.Trigger) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":  t.Spec.Broker,
		"eventing.knative.dev/trigger": t.Name,
	}
}
