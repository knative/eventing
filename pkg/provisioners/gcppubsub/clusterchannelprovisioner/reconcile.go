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

	"github.com/knative/pkg/logging"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
)

const (
	// Name is the name of the GCP PubSub ClusterChannelProvisioner.
	Name = "gcp-pubsub"
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
	ctx := context.TODO()
	ctx = logging.WithLogger(ctx, r.logger.With(zap.Any("request", request)).Sugar())

	ccp := &eventingv1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, ccp)

	// The ClusterChannelProvisioner may have been deleted since it was added to the workqueue. If
	// so, there is nothing to be done.
	if errors.IsNotFound(err) {
		logging.FromContext(ctx).Info("Could not find ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logging.FromContext(ctx).Error("Unable to Get ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterChannelProvisioner?
	if !shouldReconcile(ccp.Namespace, ccp.Name) {
		logging.FromContext(ctx).Info("Not reconciling ClusterChannelProvisioner, it is not controlled by this Controller", zap.String("APIVersion", ccp.APIVersion), zap.String("Kind", ccp.Kind), zap.String("Namespace", ccp.Namespace), zap.String("name", ccp.Name))
		return reconcile.Result{}, nil
	}
	logging.FromContext(ctx).Info("Reconciling ClusterChannelProvisioner.")

	// Modify a copy of this object, rather than the original.
	ccp = ccp.DeepCopy()

	reconcileErr := r.reconcile(ctx, ccp)
	if reconcileErr != nil {
		logging.FromContext(ctx).Info("Error reconciling ClusterChannelProvisioner", zap.Error(reconcileErr))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if err = util.UpdateClusterChannelProvisionerStatus(ctx, r.client, ccp); err != nil {
		logging.FromContext(ctx).Info("Error updating ClusterChannelProvisioner Status", zap.Error(err))
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, reconcileErr
}

// IsControlled determines if the gcp-pubsub Channel Controller should control (and therefore
// reconcile) a given object, based on that object's ClusterChannelProvisioner reference.
func IsControlled(ref *corev1.ObjectReference) bool {
	if ref != nil {
		return shouldReconcile(ref.Namespace, ref.Name)
	}
	return false
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles gcp-pubsub channels.
func shouldReconcile(namespace, name string) bool {
	return namespace == "" && name == Name
}

func (r *reconciler) reconcile(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	// We are syncing nothing! Just mark it ready.

	if ccp.DeletionTimestamp != nil {
		return nil
	}

	ccp.Status.MarkReady()
	return nil
}
