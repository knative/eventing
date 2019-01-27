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

	"github.com/knative/eventing/contrib/natss/pkg/dispatcher/dispatcher"
	"github.com/knative/eventing/pkg/provisioners"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ccpcontroller "github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	subscriptionsSupervisor *dispatcher.SubscriptionsSupervisor
}

const (
	finalizerName = controllerAgentName
)

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

	// The Channel may have been deleted since it was added to the workqueue.
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

	// Modify a copy, not the original.
	c = c.DeepCopy()

	requeue, reconcileErr := r.reconcile(ctx, c)
	if reconcileErr != nil {
		r.logger.Error("Error reconciling Channel", zap.Error(reconcileErr))
		// Note that we do not return the error here, because we want to update the Status regardless of the error.
	}

	if updateStatusErr := provisioners.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		r.logger.Error("Error updating Channel Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{
		Requeue: requeue,
	}, reconcileErr
}

func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) (bool, error) {
	if !c.DeletionTimestamp.IsZero() {
		if err := r.subscriptionsSupervisor.UpdateSubscriptions(c, true); err != nil {
			r.logger.Error("UpdateSubscriptions() failed: ", zap.Error(err))
			return false, err
		}
		provisioners.RemoveFinalizer(c, finalizerName)
		return false, nil
	}

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	if addFinalizerResult := provisioners.AddFinalizer(c, finalizerName); addFinalizerResult == provisioners.FinalizerAdded {
		return true, nil
	}

	// try to subscribe
	if err := r.subscriptionsSupervisor.UpdateSubscriptions(c, false); err != nil {
		r.logger.Error("UpdateSubscriptions() failed: ", zap.Error(err))
		return false, err
	}
	return false, nil
}
