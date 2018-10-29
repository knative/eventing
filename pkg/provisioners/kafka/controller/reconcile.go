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

	"github.com/knative/eventing/pkg/apis/eventing"
	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Provisioner resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
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
	if provisioner.Name != r.config.Name {
		return reconcile.Result{}, nil
	}

	// Only reconcile channel provisioners
	if provisioner.Spec.Reconciles.Group != eventing.GroupName || provisioner.Spec.Reconciles.Kind != "Channel" {
		return reconcile.Result{}, nil
	}

	original := provisioner.DeepCopy()

	// Reconcile this copy of the Provisioner and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(provisioner)
	if !equality.Semantic.DeepEqual(original.Status, provisioner.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		if _, err := r.updateStatus(provisioner); err != nil {
			r.logger.Info("failed to update Provisioner status", zap.Error(err))
			return reconcile.Result{}, err
		}
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(provisioner *v1alpha1.ClusterChannelProvisioner) error {
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
	// Update Status as Ready
	provisioner.Status.MarkReady()

	return nil
}

func (r *reconciler) updateStatus(provisioner *v1alpha1.ClusterChannelProvisioner) (*v1alpha1.ClusterChannelProvisioner, error) {
	newProvisioner := &v1alpha1.ClusterChannelProvisioner{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: provisioner.Namespace, Name: provisioner.Name}, newProvisioner)

	if err != nil {
		return nil, err
	}
	newProvisioner.Status = provisioner.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Provisioner resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newProvisioner); err != nil {
		return nil, err
	}
	return newProvisioner, nil
}
