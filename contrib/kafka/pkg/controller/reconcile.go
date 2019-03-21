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

package controller

import (
	"context"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/logging"
	util "github.com/knative/eventing/pkg/provisioners"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
)

const (
	// Name is the name of the kafka ClusterChannelProvisioner.
	Name = "kafka"
)

// eventingreconciler.EventingReconciler impl
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// eventingreconciler.EventingReconciler impl
func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &v1alpha1.ClusterChannelProvisioner{}
}

// eventingreconciler.Filter impl
func (r *reconciler) ShouldReconcile(ctx context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) bool {
	if obj.GetName() == Name {
		return true
	}
	logging.FromContext(ctx).Info("not reconciling ClusterChannelProvisioner, it is not controlled by this Controller")
	return false
}

// eventingreconciler.EventingReconciler impl
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	provisioner := obj.(*v1alpha1.ClusterChannelProvisioner)
	provisioner.Status.InitializeConditions()
	svc, err := util.CreateDispatcherService(ctx, r.client, provisioner)
	logger := logging.FromContext(ctx)
	if err != nil {
		logger.Info("error creating the ClusterProvisioner's K8s Service", zap.Error(err))
		return true, reconcile.Result{}, err
	}

	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, provisioner) {
		logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", provisioner), zap.Any("service", svc))
	}

	// Update Status as Ready
	provisioner.Status.MarkReady()

	return true, reconcile.Result{}, nil
}
