/*
Copyright 2019 The Knative Authors

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

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "channel-default-controller"
)

type reconciler struct {
	client client.Client
}

// Verify reconciler implements necessary interfaces
var _ eventingreconciler.EventingReconciler = &reconciler{}

// ProvideController returns a Channel controller.
// This Channel controller is a default controller for channels of all provisioner kinds
func ProvideController(mgr manager.Manager, logger *zap.Logger) (controller.Controller, error) {
	logger = logger.With(zap.String("controller", controllerAgentName))

	r, err := eventingreconciler.New(
		&reconciler{},
		logger,
		mgr.GetRecorder(controllerAgentName),
	)
	if err != nil {
		return nil, err
	}

	// Setup a new controller to Reconcile channel
	c, err := controller.New(controllerAgentName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return nil, err
	}

	// Watch channel events
	// This controller is no-op when Channels are deleted
	if err := c.Watch(
		&source.Kind{Type: &v1alpha1.Channel{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			DeleteFunc: func(event.DeleteEvent) bool {
				return false
			},
		}); err != nil {
		return nil, err
	}

	return c, nil
}

// Reconcile will check if the channel is being watched by provisioner's channel controller
// This will improve UX. See https://github.com/knative/eventing/issues/779
// eventingreconciler.EventingReconciler
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	ch := obj.(*v1alpha1.Channel)

	// Do not Initialize() Status in channel-default-controller. It will set ChannelConditionProvisionerInstalled=True
	// Directly call GetCondition(). If the Status was never initialized then GetCondition() will return nil and
	// IsUnknown() will return true
	c := ch.Status.GetCondition(v1alpha1.ChannelConditionProvisionerInstalled)

	if c.IsUnknown() {
		ch.Status.MarkProvisionerNotInstalled(
			"Provisioner not found.",
			"Specified provisioner [Name:%s Kind:%s] is not installed or not controlling the channel.",
			ch.Spec.Provisioner.Name,
			ch.Spec.Provisioner.Kind,
		)
		return true, reconcile.Result{}, nil
	}

	return false, reconcile.Result{}, nil
}

// eventingreconciler.EventingReconciler
func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &v1alpha1.Channel{}
}

// eventingreconciler.EventingReconciler
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
