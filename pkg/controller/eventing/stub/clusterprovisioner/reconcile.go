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

package clusterprovisioner

import (
	"context"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/system"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type reconciler struct {
	mgr        manager.Manager
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	cp := &eventingv1alpha1.ClusterProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, cp)

	// The ClusterProvisioner may have been deleted since it was added to the workqueue. If so,
	// there is nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find ClusterProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get ClusterProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterProvisioner?
	if !shouldReconcile(cp.Namespace, cp.Name) {
		logger.Info("Not reconciling ClusterProvisioner, it is not controlled by this Controller", zap.String("APIVersion", cp.APIVersion), zap.String("Kind", cp.Kind), zap.String("Namespace", cp.Namespace), zap.String("name", cp.Name))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling ClusterProvisioner.")

	err = r.reconcile(ctx, cp)
	if err != nil {
		logger.Info("Error reconciling ClusterProvisioner", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	updateStatusErr := r.updateClusterProvisionerStatus(ctx, cp)
	if updateStatusErr != nil {
		logger.Info("Error updating ClusterProvisioner Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

func IsControlled(ref *eventingv1alpha1.ProvisionerReference) bool {
	if ref != nil {
		return shouldReconcile(ref.Ref.Namespace, ref.Ref.Name)
	}
	return false
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterProvisioner. This Controller only handles Stub buses.
func shouldReconcile(namespace, name string) bool {
	return namespace == "" && name == "stub-bus-provisioner"
}

func (r *reconciler) reconcile(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	logger := r.logger.With(zap.Any("clusterProvisioner", cp))

	// We are syncing one thing.
	// 1. The K8s Service to talk to this Stub bus.
	//     - There is a single K8s Service for all requests going to this Stub bus.

	if cp.DeletionTimestamp != nil {
		// K8s garbage collection will delete the dispatcher service, once this ClusterProvisioner
		// is deleted, so we don't need to do anything.
		return nil
	}

	err := r.createDispatcherService(ctx, cp)
	if err != nil {
		logger.Info("Error creating the ClusterProvisioner's K8s Service", zap.Error(err))
		return err
	}

	r.setStatusReady(cp)
	return nil
}

func (r *reconciler) createDispatcherService(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	svcName := controller.ClusterBusDispatcherServiceName(cp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)

	if errors.IsNotFound(err) {
		svc = newDispatcherService(cp)
		err = r.client.Create(ctx, svc)
	}

	// If an error occurred in either Get or Create, we need to reconcile again.
	if err != nil {
		return err
	}

	// Check if this ClusterProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, cp) {
		r.logger.Warn("ClusterProvisioner's K8s Service is not owned by the ClusterProvisioner", zap.Any("clusterProvisioner", cp), zap.Any("service", svc))
	}
	return nil
}

func (r *reconciler) setStatusReady(cp *eventingv1alpha1.ClusterProvisioner) {
	cp.Status.Conditions = []duckv1alpha1.Condition{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func (r *reconciler) updateClusterProvisionerStatus(ctx context.Context, u *eventingv1alpha1.ClusterProvisioner) error {
	o := &eventingv1alpha1.ClusterProvisioner{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o)
	if err != nil {
		r.logger.Info("Error getting ClusterProvisioner for status update", zap.Error(err), zap.Any("updatedClusterProvisioner", u))
		return err
	}

	if !equality.Semantic.DeepEqual(o.Status, u.Status) {
		o.Status = u.Status
		return r.client.Update(ctx, o)
	}
	return nil
}

// newDispatcherService creates a new Service for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newDispatcherService(cp *eventingv1alpha1.ClusterProvisioner) *corev1.Service {
	labels := dispatcherLabels(cp.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherServiceName(cp.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cp, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterProvisioner",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func dispatcherLabels(cpName string) map[string]string {
	return map[string]string{
		"clusterProvisioner": cpName,
		"role":               "dispatcher",
	}
}
