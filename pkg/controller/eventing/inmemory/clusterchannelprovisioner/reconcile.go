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

package clusterchannelprovisioner

import (
	"context"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Name is the name of the in-memory channel ClusterChannelProvisioner.
	Name = "in-memory-channel"

	// Channel is the name of the Channel resource in eventing.knative.dev/v1alpha1.
	Channel = "Channel"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger
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

	ccp := &eventingv1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, ccp)

	// The ClusterChannelProvisioner may have been deleted since it was added to the workqueue. If so,
	// there is nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterChannelProvisioner?
	if !shouldReconcile(ccp.Namespace, ccp.Name) {
		logger.Info("Not reconciling ClusterChannelProvisioner, it is not controlled by this Controller", zap.String("APIVersion", ccp.APIVersion), zap.String("Kind", ccp.Kind), zap.String("Namespace", ccp.Namespace), zap.String("name", ccp.Name))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling ClusterChannelProvisioner.")

	// Modify a copy of this object, rather than the original.
	ccp = ccp.DeepCopy()

	err = r.reconcile(ctx, ccp)
	if err != nil {
		logger.Info("Error reconciling ClusterChannelProvisioner", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := r.updateClusterChannelProvisionerStatus(ctx, ccp); updateStatusErr != nil {
		logger.Info("Error updating ClusterChannelProvisioner Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// IsControlled determines if the in-memory Channel Controller should control (and therefore
// reconcile) a given object, based on that object's ClusterChannelProvisioner reference.
func IsControlled(ref *corev1.ObjectReference) bool {
	if ref != nil {
		return shouldReconcile(ref.Namespace, ref.Name)
	}
	return false
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
func shouldReconcile(namespace, name string) bool {
	return namespace == "" && name == Name
}

func (r *reconciler) reconcile(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	logger := r.logger.With(zap.Any("clusterChannelProvisioner", ccp))

	// We are syncing one thing.
	// 1. The K8s Service to talk to all in-memory Channels.
	//     - There is a single K8s Service for all requests going any in-memory Channel.

	if ccp.DeletionTimestamp != nil {
		// K8s garbage collection will delete the dispatcher service, once this ClusterChannelProvisioner
		// is deleted, so we don't need to do anything.
		return nil
	}

	if err := r.createDispatcherService(ctx, ccp); err != nil {
		logger.Info("Error creating the ClusterChannelProvisioner's K8s Service", zap.Error(err))
		return err
	}

	ccp.Status.MarkReady()
	return nil
}

func (r *reconciler) createDispatcherService(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	svcName := controller.ClusterBusDispatcherServiceName(ccp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)

	if errors.IsNotFound(err) {
		svc = newDispatcherService(ccp)
		err = r.client.Create(ctx, svc)
	}

	// If an error occurred in either Get or Create, we need to reconcile again.
	if err != nil {
		return err
	}

	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, ccp) {
		r.logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", ccp), zap.Any("service", svc))
	}
	return nil
}

func (r *reconciler) updateClusterChannelProvisionerStatus(ctx context.Context, u *eventingv1alpha1.ClusterChannelProvisioner) error {
	o := &eventingv1alpha1.ClusterChannelProvisioner{}
	if err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o); err != nil {
		r.logger.Info("Error getting ClusterChannelProvisioner for status update", zap.Error(err), zap.Any("updatedClusterChannelProvisioner", u))
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
func newDispatcherService(ccp *eventingv1alpha1.ClusterChannelProvisioner) *corev1.Service {
	labels := dispatcherLabels(ccp.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherServiceName(ccp.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ccp, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterChannelProvisioner",
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

func dispatcherLabels(ccpName string) map[string]string {
	return map[string]string{
		"clusterChannelProvisioner": ccpName,
		"role":                      "dispatcher",
	}
}
