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

	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Name is the name of NATSS ClusterChannelProvisioner.
	Name = "natss"
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
	r.logger.Info("Reconcile: ", zap.Any("request", request))

	// Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/214 is fixed.
	// The reconcile requests will include a namespace if they are triggered because of changes to the
	// objects owned by this ClusterChannelProvisioner (e.g k8s service). Since ClusterChannelProvisioner is
	// cluster-scoped we need to unset the namespace or otherwise the provisioner object cannot be looked up.
	request.NamespacedName.Namespace = ""

	ctx := context.TODO()
	ccp := &eventingv1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, ccp)

	// The ClusterChannelProvisioner may have been deleted since it was added to the workqueue. If so,
	// there is nothing to be done.
	if errors.IsNotFound(err) {
		r.logger.Warn("Could not find ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retrieved in another reconciliation.
	if err != nil {
		r.logger.Error("Unable to Get ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterChannelProvisioner?
	if !shouldReconcile(ccp.Namespace, ccp.Name) {
		r.logger.Info("Not reconciling ClusterChannelProvisioner, it is not controlled by this Controller", zap.String("APIVersion", ccp.APIVersion), zap.String("Kind", ccp.Kind), zap.String("Namespace", ccp.Namespace), zap.String("name", ccp.Name))
		return reconcile.Result{}, nil
	}
	r.logger.Info("Reconciling ClusterChannelProvisioner.")

	// Modify a copy of this object, rather than the original.
	ccp = ccp.DeepCopy()

	reconcileErr := r.reconcile(ctx, ccp)
	if reconcileErr != nil {
		r.logger.Info("Error reconciling ClusterChannelProvisioner", zap.Error(reconcileErr))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := provisioners.UpdateClusterChannelProvisionerStatus(ctx, r.client, ccp); updateStatusErr != nil {
		r.logger.Error("Error updating ClusterChannelProvisioner Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, reconcileErr
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
// ClusterChannelProvisioner. This Controller only handles Natss channels.
func shouldReconcile(namespace, name string) bool {
	return namespace == "" && name == Name
}

func (r *reconciler) reconcile(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	if ccp.DeletionTimestamp != nil {
		// K8s garbage collection will delete the dispatcher service, once this ClusterChannelProvisioner
		// is deleted, so we don't need to do anything.
		return nil
	}

	svc, err := provisioners.CreateDispatcherService(ctx, r.client, ccp)
	if err != nil {
		r.logger.Error("Error creating the ClusterChannelProvisioner's Dispatcher", zap.Error(err))
		return err
	}
	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, ccp) {
		r.logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", ccp), zap.Any("service", svc))
	}

	ccp.Status.MarkReady()
	return nil
}
