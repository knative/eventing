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

	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
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
	client client.Client
}

// Verify the struct implements eventingreconciler.EventingReconciler
var _ eventingreconciler.EventingReconciler = &reconciler{}
var _ eventingreconciler.Filter = &reconciler{}

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
	return shouldReconcile(obj.GetNamespace(), obj.GetName())
}

// eventingreconciler.EventingReconciler impl
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	ccp := obj.(*eventingv1alpha1.ClusterChannelProvisioner)
	logger := logging.FromContext(ctx)
	svc, err := provisioners.CreateDispatcherService(ctx, r.client, ccp)
	if err != nil {
		logger.Error("Error creating the ClusterChannelProvisioner's Dispatcher", zap.Error(err))
		return true, reconcile.Result{}, err
	}
	// Check if this ClusterChannelProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, ccp) {
		logger.Warn("ClusterChannelProvisioner's K8s Service is not owned by the ClusterChannelProvisioner", zap.Any("clusterChannelProvisioner", ccp), zap.Any("service", svc))
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
// ClusterChannelProvisioner. This Controller only handles Natss channels.
func shouldReconcile(namespace, name string) bool {
	return namespace == "" && name == Name
}
