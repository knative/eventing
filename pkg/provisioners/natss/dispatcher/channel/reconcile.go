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

	"github.com/knative/eventing/pkg/provisioners"
	"github.com/knative/eventing/pkg/provisioners/natss/dispatcher/dispatcher"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/natss/controller/clusterchannelprovisioner"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type reconciler struct {
	client   client.Client
	recorder record.EventRecorder
	logger   *zap.Logger

	subscriptionsSupervisor *dispatcher.SubscriptionsSupervisor
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	r.logger.Info("request:" + request.String())

	// TODO: use this to store the logger and set a deadline
	ctx := context.TODO()
	logger := r.logger.With(zap.Any("request", request))

	c := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, request.NamespacedName, c)

	// The Channel may have been deleted since it was added to the workqueue. If so, there is
	// nothing to be done.
	if errors.IsNotFound(err) {
		logger.Error("Could not find Channel", zap.Error(err))
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
	logger.Sugar().Infof("Reconciling Channel: %+v", c)

	// Modify a copy, not the original.
	c = c.DeepCopy()

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Error("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := provisioners.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		logger.Error("Error updating Channel Status", zap.Error(updateStatusErr))
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
	r.logger.Sugar().Infof("reconcile() channel: %+v", c)

	logger := r.logger.With(zap.Any("channel", c))

	c.Status.InitializeConditions()

	// We are syncing Channel subscriptions
	if err := r.syncChannel(ctx); err != nil {
		logger.Error("Error updating syncing the Channel config", zap.Error(err))
		return err
	}

	//	c.Status.MarkProvisioned()
	return nil
}

func (r *reconciler) syncChannel(ctx context.Context) error {
	channels, err := r.listAllChannels(ctx)
	if err != nil {
		r.logger.Error("Unable to list channels", zap.Error(err))
		return err
	}

	// try to subscribe
	for _, c := range channels {
		r.logger.Info("syncChannel() channel:" + c.Name + "." + c.Namespace)
		if err := r.subscriptionsSupervisor.UpdateSubscriptions(c); err != nil {
			r.logger.Error("UpdateSubscriptions() failed: ", zap.Error(err))
		}
	}
	return nil

}
func (r *reconciler) listAllChannels(ctx context.Context) ([]eventingv1alpha1.Channel, error) {
	channels := make([]eventingv1alpha1.Channel, 0)

	opts := &client.ListOptions{
		// TODO this is here because the fake client needs it. Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Channel",
			},
		},
	}
	for {
		cl := &eventingv1alpha1.ChannelList{}
		if err := r.client.List(ctx, opts, cl); err != nil {
			return nil, err
		}

		for _, c := range cl.Items {
			if r.shouldReconcile(&c) {
				channels = append(channels, c)
			}
		}
		if cl.Continue != "" {
			opts.Raw.Continue = cl.Continue
		} else {
			return channels, nil
		}
	}
}
