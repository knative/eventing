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
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
)

const (
	// Name is the name of the kafka ClusterChannelProvisioner.
	Name = "kafka"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Provisioner resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	r.logger.Info("reconciling ClusterChannelProvisioner", zap.Any("request", request))
	provisioner := &v1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(context.TODO(), request.NamespacedName, provisioner)

	if errors.IsNotFound(err) {
		r.logger.Info("could not find ClusterChannelProvisioner", zap.Any("request", request))
		return reconcile.Result{}, nil
	}

	if err != nil {
		r.logger.Error("could not fetch ClusterChannelProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Skip channel provisioners that we don't manage
	if provisioner.Name != Name {
		r.logger.Info("not reconciling ClusterChannelProvisioner, it is not controlled by this Controller", zap.Any("request", request))
		return reconcile.Result{}, nil
	}

	newProvisioner := provisioner.DeepCopy()

	// Reconcile this copy of the Provisioner and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(ctx, newProvisioner)
	if err != nil {
		r.logger.Info("error reconciling ClusterProvisioner", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}
	if updateStatusErr := util.UpdateClusterChannelProvisionerStatus(ctx, r.client, newProvisioner); updateStatusErr != nil {
		r.logger.Info("error updating ClusterChannelProvisioner Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(ctx context.Context, provisioner *v1alpha1.ClusterChannelProvisioner) error {
	// See if the provisioner has been deleted
	accessor, err := meta.Accessor(provisioner)
	if err != nil {
		r.logger.Info("failed to get metadata", zap.Error(err))
		return err
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		r.logger.Info(fmt.Sprintf("DeletionTimestamp: %v", deletionTimestamp))
		return nil
	}

	provisioner.Status.InitializeConditions()

	svc, err := util.CreateDispatcherService(ctx, r.client, provisioner)
	if err != nil {
		r.logger.Info("error creating the ClusterProvisioner's K8s Service", zap.Error(err))
		return err
	}

	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, provisioner) {
		r.logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", provisioner), zap.Any("service", svc))
	}

	// Update Status as Ready
	provisioner.Status.MarkReady()

	return nil
}
