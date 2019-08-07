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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	util "knative.dev/eventing/pkg/provisioners"
)

const (
	// Channel is the name of the Channel resource in eventing.knative.dev/v1alpha1.
	Channel = "Channel"

	// Name of the corev1.Events emitted from the reconciliation process
	ccpReconciled          = "CcpReconciled"
	ccpUpdateStatusFailed  = "CcpUpdateStatusFailed"
	k8sServiceCreateFailed = "K8sServiceCreateFailed"
	k8sServiceDeleteFailed = "K8sServiceDeleteFailed"
)

var (
	// provisionerNames contains the list of provisioners' names served by this controller
	provisionerNames = []string{"in-memory"}
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
	// TODO use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	// Workaround until https://github.com/kubernetes-sigs/controller-runtime/issues/214 is fixed.
	// The reconcile requests will include a namespace if they are triggered because of changes to the
	// objects owned by this ClusterChannelProvisioner (e.g k8s service). Since ClusterChannelProvisioner is
	// cluster-scoped we need to unset the namespace or otherwise the provisioner object cannot be looked up.
	request.NamespacedName.Namespace = ""

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
	} else {
		logger.Info("ClusterChannelProvisioner reconciled")
		r.recorder.Eventf(ccp, corev1.EventTypeNormal, ccpReconciled, "ClusterChannelProvisioner reconciled: %q", ccp.Name)
	}

	if updateStatusErr := util.UpdateClusterChannelProvisionerStatus(ctx, r.client, ccp); updateStatusErr != nil {
		logger.Info("Error updating ClusterChannelProvisioner Status", zap.Error(updateStatusErr))
		r.recorder.Eventf(ccp, corev1.EventTypeWarning, ccpUpdateStatusFailed, "Failed to update ClusterChannelProvisioner's status: %v", err)
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
	for _, p := range provisionerNames {
		if namespace == "" && name == p {
			return true
		}
	}
	return false
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

	svc, err := util.CreateDispatcherService(ctx, r.client, ccp, setDispatcherServiceSelector())

	if err != nil {
		logger.Info("Error creating the ClusterChannelProvisioner's K8s Service", zap.Error(err))
		r.recorder.Eventf(ccp, corev1.EventTypeWarning, k8sServiceCreateFailed, "Failed to reconcile ClusterChannelProvisioner's K8s Service: %v", err)
		return err
	}

	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, ccp) {
		logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", ccp), zap.Any("service", svc))
	}

	ccp.Status.MarkReady()
	return nil
}

func setDispatcherServiceSelector() util.ServiceOption {
	return func(svc *v1.Service) error {
		// There used to be an "in-memory-channel" provisioner. It was removed in 0.7, but it was
		// the original, so the dispatcher is still labeled with its name.
		svc.Spec.Selector = util.DispatcherLabels("in-memory-channel")
		return nil
	}
}
