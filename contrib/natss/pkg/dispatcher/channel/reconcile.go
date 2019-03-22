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
	"github.com/knative/eventing/pkg/logging"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ccpcontroller "github.com/knative/eventing/contrib/natss/pkg/controller/clusterchannelprovisioner"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
)

type reconciler struct {
	client                  client.Client
	subscriptionsSupervisor *dispatcher.SubscriptionsSupervisor
}

const (
	finalizerName = controllerAgentName
)

// Verify the struct implements eventingreconciler.EventingReconciler
var _ eventingreconciler.EventingReconciler = &reconciler{}
var _ eventingreconciler.Filter = &reconciler{}
var _ eventingreconciler.Finalizer = &reconciler{}

// eventingreconciler.EventingReconciler
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// eventingreconciler.EventingReconciler
func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &eventingv1alpha1.Channel{}
}

// eventingreconciler.EventingReconciler
func (r *reconciler) ShouldReconcile(_ context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) bool {
	c := obj.(*eventingv1alpha1.Channel)
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

// eventingreconciler.EventingReconciler
// ReconcileResource never updates channel status. It
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	c := obj.(*eventingv1alpha1.Channel)

	// try to subscribe
	if err := r.subscriptionsSupervisor.UpdateSubscriptions(c, false); err != nil {
		logging.FromContext(ctx).Error("UpdateSubscriptions() failed: ", zap.Error(err))
		return false, reconcile.Result{}, err
	}
	return false, reconcile.Result{}, nil
}

func (r *reconciler) OnDelete(ctx context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) error {
	c := obj.(*eventingv1alpha1.Channel)
	if err := r.subscriptionsSupervisor.UpdateSubscriptions(c, true); err != nil {
		logging.FromContext(ctx).Error("UpdateSubscriptions() failed: ", zap.Error(err))
		return err
	}
	return nil
}
