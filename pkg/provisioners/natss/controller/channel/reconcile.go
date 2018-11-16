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

	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/natss/controller/clusterchannelprovisioner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	finalizerName = controllerAgentName
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
	r.logger.Info("Reconcile: ", zap.Any("request", request))

	ctx := context.TODO()
	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		r.logger.Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		r.logger.Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		r.logger.Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	//r.logger.Info("Reconciling Channel:", zap.Any("channel", c))

	// Modify a copy, not the original.
	c = c.DeepCopy()
	err = r.reconcile(ctx, c)
	if err != nil {
		r.logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := provisioners.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		r.logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	c.Status.InitializeConditions()

	// We are syncing two things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.

	if c.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service and VirtualService for this channel.
		// We use a finalizer to ensure the channel config has been synced.
		provisioners.RemoveFinalizer(c, finalizerName)
		return nil
	}

	provisioners.AddFinalizer(c, finalizerName)

	svc, err := provisioners.CreateK8sService(ctx, r.client, c)
	if err != nil {
		r.logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		return err
	}
	// Check if this Channel is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, c) {
		r.logger.Warn("Channel's K8s Service is not owned by the Channel", zap.Any("channel", c), zap.Any("service", svc))
	}
	c.Status.SetAddress(controller.ServiceHostName(svc.Name, svc.Namespace))

	virtualService, err := provisioners.CreateVirtualService(ctx, r.client, c)
	if err != nil {
		r.logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}
	if !metav1.IsControlledBy(virtualService, c) {
		r.logger.Warn("VirtualService not owned by Channel", zap.Any("channel", c), zap.Any("virtualService", virtualService))
	}

	c.Status.MarkProvisioned()
	return nil
}
