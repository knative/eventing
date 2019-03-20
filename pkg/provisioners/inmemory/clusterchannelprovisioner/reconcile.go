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
	"fmt"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	util "github.com/knative/eventing/pkg/provisioners"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	"github.com/knative/pkg/system"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Channel is the name of the Channel resource in eventing.knative.dev/v1alpha1.
	Channel = "Channel"

	// Name of the corev1.Events emitted from the reconciliation process
	k8sServiceCreateFailed = "K8sServiceCreateFailed"
	k8sServiceDeleteFailed = "K8sServiceDeleteFailed"
)

var (
	// provisionerNames contains the list of provisioners' names served by this controller
	provisionerNames = []string{"in-memory-channel", "in-memory"}
)

type reconciler struct {
	client client.Client
}

// Verify reconciler implements necessary interfaces
var _ eventingreconciler.EventingReconciler = &reconciler{}

// eventingreconciler.EventingReconciler
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// eventingreconciler.EventingReconciler
func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &v1alpha1.ClusterChannelProvisioner{}
}

// eventingreconciler.EventingReconciler
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
	ccp := obj.(*eventingv1alpha1.ClusterChannelProvisioner)

	// We are syncing one thing.
	// 1. The K8s Service to talk to all in-memory Channels.
	//     - There is a single K8s Service for all requests going any in-memory Channel.

	svc, err := util.CreateDispatcherService(ctx, r.client, ccp)
	logger := logging.FromContext(ctx)
	if err != nil {
		logger.Info("Error creating the ClusterChannelProvisioner's K8s Service", zap.Error(err))
		recorder.Eventf(ccp, corev1.EventTypeWarning, k8sServiceCreateFailed, "Failed to reconcile ClusterChannelProvisioner's K8s Service: %v", err)
		return true, reconcile.Result{}, err
	}

	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, ccp) {
		logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", ccp), zap.Any("service", svc))
	}

	// The name of the svc has changed since version 0.2.1. Hence, delete old dispatcher service (in-memory-channel-clusterbus)
	// that was created previously in version 0.2.0 to ensure backwards compatibility.
	err = r.deleteOldDispatcherService(ctx, ccp)
	if err != nil {
		logger.Info("Error deleting the old ClusterChannelProvisioner's K8s Service", zap.Error(err))
		recorder.Eventf(ccp, corev1.EventTypeWarning, k8sServiceDeleteFailed, "Failed to delete the old ClusterChannelProvisioner's K8s Service: %v", err)
		return true, reconcile.Result{}, err
	}

	ccp.Status.MarkReady()
	return true, reconcile.Result{}, nil

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
// eventingreconciler.EventingReconciler
func (r *reconciler) ShouldReconcile(_ context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) bool {
	return shouldReconcile(obj.GetNamespace(), obj.GetName())
}
func shouldReconcile(namespace, name string) bool {
	for _, p := range provisionerNames {
		if namespace == "" && name == p {
			return true
		}
	}
	return false
}

func (r *reconciler) deleteOldDispatcherService(ctx context.Context, ccp *eventingv1alpha1.ClusterChannelProvisioner) error {
	svcName := fmt.Sprintf("%s-clusterbus", ccp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	return r.client.Delete(ctx, svc)
}
