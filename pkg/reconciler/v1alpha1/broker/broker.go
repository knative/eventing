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
	"time"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/reconciler/v1alpha1/broker/resources"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
	controllerAgentName = "broker-controller"

	// Name of the corev1.Events emitted from the reconciliation process.
	brokerReconciled         = "BrokerReconciled"
	brokerUpdateStatusFailed = "BrokerUpdateStatusFailed"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder

	logger *zap.Logger

	ingressImage              string
	ingressServiceAccountName string
	filterImage               string
	filterServiceAccountName  string
}

// Verify the struct implements reconcile.Reconciler.
var _ reconcile.Reconciler = &reconciler{}

type ReconcilerArgs struct {
	IngressImage              string
	IngressServiceAccountName string
	FilterImage               string
	FilterServiceAccountName  string
}

// ProvideController returns a function that returns a Broker controller.
func ProvideController(args ReconcilerArgs) func(manager.Manager, *zap.Logger) (controller.Controller, error) {
	return func(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
		// Setup a new controller to Reconcile Brokers.
		c, err := controller.New(controllerAgentName, mgr, controller.Options{
			Reconciler: &reconciler{
				recorder: mgr.GetRecorder(controllerAgentName),
				logger:   logger,

				ingressImage:              args.IngressImage,
				ingressServiceAccountName: args.IngressServiceAccountName,
				filterImage:               args.FilterImage,
				filterServiceAccountName:  args.FilterServiceAccountName,
			},
		})
		if err != nil {
			return nil, err
		}

		// Watch Brokers.
		if err = c.Watch(&source.Kind{Type: &v1alpha1.Broker{}}, &handler.EnqueueRequestForObject{}); err != nil {
			return nil, err
		}

		// Watch all the resources that the Broker reconciles.
		for _, t := range []runtime.Object{&v1alpha1.Channel{}, &corev1.Service{}, &v1.Deployment{}} {
			err = c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{OwnerType: &v1alpha1.Broker{}, IsController: true})
			if err != nil {
				return nil, err
			}
		}

		return c, nil
	}
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Broker resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)))

	broker := &v1alpha1.Broker{}
	err := r.client.Get(ctx, request.NamespacedName, broker)

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
	result, reconcileErr := r.reconcile(ctx, broker)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling Broker", zap.Error(reconcileErr))
	} else if result.Requeue || result.RequeueAfter > 0 {
		logging.FromContext(ctx).Debug("Broker reconcile requeuing")
	} else {
		logging.FromContext(ctx).Debug("Broker reconciled")
		r.recorder.Event(broker, corev1.EventTypeNormal, brokerReconciled, "Broker reconciled")
	}

	if _, err = r.updateStatus(broker); err != nil {
		logging.FromContext(ctx).Error("Failed to update Broker status", zap.Error(err))
		r.recorder.Eventf(broker, corev1.EventTypeWarning, brokerUpdateStatusFailed, "Failed to update Broker's status: %v", err)
		return reconcile.Result{}, err
	}

	// Requeue if the resource is not ready:
	return result, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, b *v1alpha1.Broker) (reconcile.Result, error) {
	b.Status.InitializeConditions()

	// 1. Channel is created for all events.
	// 2. Filter Deployment.
	// 3. Ingress Deployment.
	// 4. K8s Services that point at the Deployments.

	if b.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return reconcile.Result{}, nil
	}

	c, err := r.reconcileChannel(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling the channel", zap.Error(err))
		b.Status.MarkChannelFailed(err)
		return reconcile.Result{}, err
	} else if c.Status.Address.Hostname == "" {
		logging.FromContext(ctx).Info("Channel is not yet ready", zap.Any("c", c))
		// Give the Channel some time to get its address. One second was chosen arbitrarily.
		return reconcile.Result{RequeueAfter: time.Second}, nil
	}
	b.Status.MarkChannelReady()

	_, err = r.reconcileFilterDeployment(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Deployment", zap.Error(err))
		b.Status.MarkFilterFailed(err)
		return reconcile.Result{}, err
	}
	_, err = r.reconcileFilterService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling filter Service", zap.Error(err))
		b.Status.MarkFilterFailed(err)
		return reconcile.Result{}, err
	}
	b.Status.MarkFilterReady()

	_, err = r.reconcileIngressDeployment(ctx, b, c)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Deployment", zap.Error(err))
		b.Status.MarkIngressFailed(err)
		return reconcile.Result{}, err
	}

	svc, err := r.reconcileIngressService(ctx, b)
	if err != nil {
		logging.FromContext(ctx).Error("Problem reconciling ingress Service", zap.Error(err))
		b.Status.MarkIngressFailed(err)
		return reconcile.Result{}, err
	}
	b.Status.MarkIngressReady()
	b.Status.SetAddress(names.ServiceHostName(svc.Name, svc.Namespace))

	return reconcile.Result{}, nil
}

// updateStatus may in fact update the broker's finalizers in addition to the status.
func (r *reconciler) updateStatus(broker *v1alpha1.Broker) (*v1alpha1.Broker, error) {
	ctx := context.TODO()
	objectKey := client.ObjectKey{Namespace: broker.Namespace, Name: broker.Name}
	latestBroker := &v1alpha1.Broker{}

	if err := r.client.Get(ctx, objectKey, latestBroker); err != nil {
		return nil, err
	}

	brokerChanged := false

	if !equality.Semantic.DeepEqual(latestBroker.Finalizers, broker.Finalizers) {
		latestBroker.SetFinalizers(broker.ObjectMeta.Finalizers)
		if err := r.client.Update(ctx, latestBroker); err != nil {
			return nil, err
		}
		brokerChanged = true
	}

	if equality.Semantic.DeepEqual(latestBroker.Status, broker.Status) {
		return latestBroker, nil
	}

	if brokerChanged {
		// Re-fetch.
		latestBroker = &v1alpha1.Broker{}
		if err := r.client.Get(ctx, objectKey, latestBroker); err != nil {
			return nil, err
		}
	}

	latestBroker.Status = broker.Status
	if err := r.client.Status().Update(ctx, latestBroker); err != nil {
		return nil, err
	}

	return latestBroker, nil
}

// reconcileFilterDeployment reconciles Broker's 'b' filter deployment.
func (r *reconciler) reconcileFilterDeployment(ctx context.Context, b *v1alpha1.Broker) (*v1.Deployment, error) {
	expected := resources.MakeFilterDeployment(&resources.FilterArgs{
		Broker:             b,
		Image:              r.filterImage,
		ServiceAccountName: r.filterServiceAccountName,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileFilterService reconciles Broker's 'b' filter service.
func (r *reconciler) reconcileFilterService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	expected := resources.MakeFilterService(b)
	return r.reconcileService(ctx, expected)
}

// reconcileChannel reconciles Broker's 'b' underlying channel.
func (r *reconciler) reconcileChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	c, err := r.getChannel(ctx, b)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		c = newChannel(b)
		err = r.client.Create(ctx, c)
		if err != nil {
			return nil, err
		}
		return c, nil
	} else if err != nil {
		return nil, err
	}

	// TODO Determine if we want to update spec (maybe just args?).
	// Update Channel if it has changed. Note that we need to both ignore the real Channel's
	// subscribable section and if we need to update the real Channel, retain it.
	//expected.Spec.Subscribable = c.Spec.Subscribable
	//if !equality.Semantic.DeepDerivative(expected.Spec, c.Spec) {
	//	c.Spec = expected.Spec
	//	err = r.client.Update(ctx, c)
	//	if err != nil {
	//		return nil, err
	//	}
	//}
	return c, nil
}

// getChannel returns the Channel object for Broker 'b' if exists, otherwise it returns an error.
func (r *reconciler) getChannel(ctx context.Context, b *v1alpha1.Broker) (*v1alpha1.Channel, error) {
	list := &v1alpha1.ChannelList{}
	opts := &runtimeclient.ListOptions{
		Namespace:     b.Namespace,
		LabelSelector: labels.SelectorFromSet(ChannelLabels(b)),
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

// newChannel creates a new Channel for Broker 'b'.
func newChannel(b *v1alpha1.Broker) *v1alpha1.Channel {
	var spec v1alpha1.ChannelSpec
	if b.Spec.ChannelTemplate != nil {
		spec = *b.Spec.ChannelTemplate
	}

	return &v1alpha1.Channel{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    b.Namespace,
			GenerateName: fmt.Sprintf("%s-broker-", b.Name),
			Labels:       ChannelLabels(b),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(b, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Broker",
				}),
			},
		},
		Spec: spec,
	}
}

func ChannelLabels(b *v1alpha1.Broker) map[string]string {
	return map[string]string{
		"eventing.knative.dev/broker":           b.Name,
		"eventing.knative.dev/brokerEverything": "true",
	}
}

// reconcileDeployment reconciles the K8s Deployment 'd'.
func (r *reconciler) reconcileDeployment(ctx context.Context, d *v1.Deployment) (*v1.Deployment, error) {
	name := types.NamespacedName{
		Namespace: d.Namespace,
		Name:      d.Name,
	}
	current := &v1.Deployment{}
	err := r.client.Get(ctx, name, current)
	if k8serrors.IsNotFound(err) {
		err = r.client.Create(ctx, d)
		if err != nil {
			return nil, err
		}
		return d, nil
	} else if err != nil {
		return nil, err
	}

	if !equality.Semantic.DeepDerivative(d.Spec, current.Spec) {
		current.Spec = d.Spec
		err = r.client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// reconcileService reconciles the K8s Service 'svc'.
func (r *reconciler) reconcileService(ctx context.Context, svc *corev1.Service) (*corev1.Service, error) {
	name := types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      svc.Name,
	}
	current := &corev1.Service{}
	err := r.client.Get(ctx, name, current)
	if k8serrors.IsNotFound(err) {
		err = r.client.Create(ctx, svc)
		if err != nil {
			return nil, err
		}
		return svc, nil
	} else if err != nil {
		return nil, err
	}

	// spec.clusterIP is immutable and is set on existing services. If we don't set this to the same value, we will
	// encounter an error while updating.
	svc.Spec.ClusterIP = current.Spec.ClusterIP
	if !equality.Semantic.DeepDerivative(svc.Spec, current.Spec) {
		current.Spec = svc.Spec
		err = r.client.Update(ctx, current)
		if err != nil {
			return nil, err
		}
	}
	return current, nil
}

// reconcileIngressDeployment reconciles the Ingress Deployment.
func (r *reconciler) reconcileIngressDeployment(ctx context.Context, b *v1alpha1.Broker, c *v1alpha1.Channel) (*v1.Deployment, error) {
	expected := resources.MakeIngress(&resources.IngressArgs{
		Broker:             b,
		Image:              r.ingressImage,
		ServiceAccountName: r.ingressServiceAccountName,
		ChannelAddress:     c.Status.Address.Hostname,
	})
	return r.reconcileDeployment(ctx, expected)
}

// reconcileIngressService reconciles the Ingress Service.
func (r *reconciler) reconcileIngressService(ctx context.Context, b *v1alpha1.Broker) (*corev1.Service, error) {
	expected := resources.MakeIngressService(b)
	return r.reconcileService(ctx, expected)
}
