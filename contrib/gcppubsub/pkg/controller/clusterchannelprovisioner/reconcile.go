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
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// Name is the name of the GCP PubSub ClusterChannelProvisioner.
	Name = "gcp-pubsub"

	// Name of the corev1.Events emitted from the reconciliation process
	ccpReconciled         = "CcpReconciled"
	ccpReconcileFailed    = "CcpReconcileFailed"
	ccpUpdateStatusFailed = "CcpUpdateStatusFailed"
)

type reconciler struct {
	client client.Client
}

// Verify the struct implements eventingreconciler.EventingReconciler
var _ eventingreconciler.EventingReconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &eventingv1alpha1.ClusterChannelProvisioner{}
}

func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	ccp := obj.(*eventingv1alpha1.ClusterChannelProvisioner)
	ccp.Status.MarkReady()
	return true, reconcile.Result{}, nil
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
