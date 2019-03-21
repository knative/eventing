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

	"github.com/knative/eventing/pkg/logging"
	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/reconciler/names"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ccpcontroller "github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
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
	return &eventingv1alpha1.Channel{}
}

// eventingreconciler.Filter  impl
func (r *reconciler) ShouldReconcile(_ context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) bool {
	c := obj.(*eventingv1alpha1.Channel)
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	c := obj.(*eventingv1alpha1.Channel)
	logger := logging.FromContext(ctx)
	ccp, err := r.getClusterChannelProvisioner()
	if err != nil {
		logger.Error("Unable to Get Cluster Channel Provisioner", zap.Error(err))
		return false, reconcile.Result{}, err
	}
	c.Status.InitializeConditions()
	var reconcileErr error
	if ccp.Status.IsReady() {
		// Reconcile this copy of the Channel and then write back any status
		// updates regardless of whether the reconcile error out.
		reconcileErr = r.reconcile(ctx, c)
		if reconcileErr != nil {
			logger.Info("Error reconciling Channel", zap.Error(reconcileErr))
		}
	} else {
		c.Status.MarkNotProvisioned("NotProvisioned", "ClusterChannelProvisioner %s is not ready", ccpcontroller.Name)
		reconcileErr = fmt.Errorf("ClusterChannelProvisioner %s is not ready", ccpcontroller.Name)
	}

	return true, reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	// We are syncing two things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	logger := logging.FromContext(ctx)
	svc, err := provisioners.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		return err
	}
	c.Status.SetAddress(names.ServiceHostName(svc.Name, svc.Namespace))

	_, err = provisioners.CreateVirtualService(ctx, r.client, c, svc)
	if err != nil {
		logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}

	c.Status.MarkProvisioned()
	return nil
}

func (r *reconciler) getClusterChannelProvisioner() (*eventingv1alpha1.ClusterChannelProvisioner, error) {
	ccp := &eventingv1alpha1.ClusterChannelProvisioner{}
	objKey := client.ObjectKey{
		Name: ccpcontroller.Name,
	}
	if err := r.client.Get(context.Background(), objKey, ccp); err != nil {
		return nil, err
	}
	return ccp, nil
}
