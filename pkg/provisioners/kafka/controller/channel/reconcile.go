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

package channel

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Channel resource
// with the current status of the resource.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.log.Info("Reconciling channel", "request", request)
	channel := &v1alpha1.Channel{}
	err := r.client.Get(context.TODO(), request.NamespacedName, channel)

	if errors.IsNotFound(err) {
		r.log.Info("could not find channel", "request", request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		r.log.Error(err, "could not fetch Channel", "request", request)
		return reconcile.Result{}, err
	}

	original := channel.DeepCopy()

	// Reconcile this copy of the Channel and then write back any status
	// updates regardless of whether the reconcile error out.
	err = r.reconcile(channel)
	if !equality.Semantic.DeepEqual(original.Status, channel.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
		if _, err := r.updateStatus(channel); err != nil {
			r.log.Info("Failed to update channel status", "error", err)
			return reconcile.Result{}, err
		}
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, err
}

func (r *reconciler) reconcile(channel *v1alpha1.Channel) error {
	// See if the channel has been deleted
	accessor, err := meta.Accessor(channel)
	if err != nil {
		r.log.Info("failed to get metadata", "error", err)
		return err
	}
	deletionTimestamp := accessor.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		r.log.Info(fmt.Sprintf("DeletionTimestamp: %v", deletionTimestamp))
		//TODO: Handle deletion
		return nil
	}

	// Skip Channel as it is not targeting any provisioner
	if channel.Spec.Provisioner == nil || channel.Spec.Provisioner.Ref == nil {
		return nil
	}

	// Skip channel not managed by this provisioner
	provisionerRef := channel.Spec.Provisioner.Ref
	clusterProvisioner, err := r.getClusterProvisioner()
	if err != nil {
		return err
	}
	// TODO: Is there a better way to compare ref?
	if provisionerRef.Name != clusterProvisioner.Name || provisionerRef.Namespace != clusterProvisioner.Namespace {
		return nil
	}

	// The provisioner must be ready
	if !clusterProvisioner.Status.IsReady() {
		channel.Status.MarkAsNotProvisioned("NotProvisioned", "ClusterProvisioner %s is not ready", clusterProvisioner.Name)
		return fmt.Errorf("ClusterProvisioner %s is not ready", clusterProvisioner.Name)
	}

	// TODO: provision channel
	channel.Status.InitializeConditions()
	channel.Status.MarkAsNotProvisioned("NotProvisioned", "NotImplemented")
	return nil
}

func (r *reconciler) getClusterProvisioner() (*v1alpha1.ClusterProvisioner, error) {
	clusterProvisioner := &v1alpha1.ClusterProvisioner{}
	objKey := client.ObjectKey{
		Name: r.config.Name,
	}
	if err := r.client.Get(context.TODO(), objKey, clusterProvisioner); err != nil {
		return nil, err
	}
	return clusterProvisioner, nil

}

func (r *reconciler) updateStatus(channel *v1alpha1.Channel) (*v1alpha1.Channel, error) {
	newChannel := &v1alpha1.Channel{}
	err := r.client.Get(context.TODO(), client.ObjectKey{Namespace: channel.Namespace, Name: channel.Name}, newChannel)

	if err != nil {
		return nil, err
	}
	newChannel.Status = channel.Status

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Channel resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err = r.client.Update(context.TODO(), newChannel); err != nil {
		return nil, err
	}
	return newChannel, nil
}
