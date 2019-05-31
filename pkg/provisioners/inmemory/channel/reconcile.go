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

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/inmemory/clusterchannelprovisioner"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/pkg/apis"
)

const (
	finalizerName = controllerAgentName
	// Name of the corev1.Events emitted from the reconciliation process
	channelReconciled         = "ChannelReconciled"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
	k8sServiceCreateFailed    = "K8sServiceCreateFailed"
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
	// TODO: use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find Channel", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get Channel", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this Channel?
	if !r.shouldReconcile(c) {
		logger.Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("ref", c.Spec))
		return reconcile.Result{}, nil
	}
	logger.Info("Reconciling Channel")

	// Finalizer needs to be removed (even though no finalizers are added) to maintain backwards
	// compatibility with v0.5 in which a finalizer was added. Or else channels will not get deleted
	// after upgrading to 0.6+.
	if result := util.RemoveFinalizer(c, finalizerName); result == util.FinalizerRemoved {
		err = r.client.Update(ctx, c)
		if err != nil {
			logger.Info("Failed to remove finalizer", zap.Error(err))
			return reconcile.Result{}, err
		}
		logger.Info("Channel reconciled. Finalizer Removed")
		r.recorder.Eventf(c, corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %q. Finalizer removed.", c.Name)
		return reconcile.Result{Requeue: true}, nil
	}

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	} else {
		logger.Info("Channel reconciled")
		r.recorder.Eventf(c, corev1.EventTypeNormal, channelReconciled, "Channel reconciled: %q", c.Name)
	}

	if updateStatusErr := r.client.Status().Update(ctx, c); updateStatusErr != nil {
		logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
		r.recorder.Eventf(c, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update Channel's status: %v", err)
		return reconcile.Result{}, updateStatusErr
	}
	return reconcile.Result{}, err
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	logger := r.logger.With(zap.Any("channel", c))

	c.Status.InitializeConditions()

	// We are syncing K8s Service to talk to this Channel.
	svc, err := util.CreateK8sService(ctx, r.client, c, util.ExternalService(c))
	if err != nil {
		logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		r.recorder.Eventf(c, corev1.EventTypeWarning, k8sServiceCreateFailed, "Failed to reconcile Channel's K8s Service: %v", err)
		return err
	}

	c.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   names.ServiceHostName(svc.Name, svc.Namespace),
	})

	c.Status.MarkProvisioned()
	return nil
}
