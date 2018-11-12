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

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/gcppubsub/clusterchannelprovisioner"
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

	// Modify a copy, not the original.
	c = c.DeepCopy()

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	if updateStatusErr := util.UpdateChannel(ctx, r.client, c); updateStatusErr != nil {
		logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
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

	// We are syncing three things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	// 3. The Gateway Deployment that is running in the same namespace as the Channel.

	if c.DeletionTimestamp != nil {
		// K8s garbage collection will delete the K8s service and VirtualService for this channel.
		return nil
	}

	svc, err := util.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		return err
	}

	// Check if this Channel is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, c) {
		logger.Warn("Channel's K8s Service is not owned by the Channel", zap.Any("channel", c), zap.Any("service", svc))
	}

	c.Status.SetAddress(controller.ServiceHostName(svc.Name, svc.Namespace))

	virtualService, err := util.CreateVirtualService(ctx, r.client, c) ///////////////////////////////////////////////////////////////// Need to replace to point at the namespaced dispatcher.

	if err != nil {
		logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}

	// If the Virtual Service is not controlled by this Channel, we should log a warning, but don't
	// consider it an error.
	if !metav1.IsControlledBy(virtualService, c) {
		logger.Warn("VirtualService not owned by Channel", zap.Any("channel", c), zap.Any("virtualService", virtualService))
	}

	_, err = r.createGateway(ctx, c.Namespace, logger)
	if err != nil {
		logger.Info("Error creating the Channel's Gateway", zap.Error(err))
	}

	c.Status.MarkProvisioned()
	return nil
}

func (r *reconciler) createGateway(ctx context.Context, namespace string, logger *zap.Logger) (*v1.Deployment, error) {
	gateway, err := r.getGateway(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if gateway != nil {
		logger.Info("Re-using existing gateway", zap.Any("gateway", gateway))
		return gateway, nil
	}

	gateway = r.newGateway(ctx, namespace)
	err = r.client.Create(ctx, gateway)
	return gateway, err
}

func (r *reconciler) getGateway(ctx context.Context, namespace string) (*v1.Deployment, error) {
	dl := &v1.DeploymentList{}
	err := r.client.List(ctx, &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: r.getLabelSelector(),
		// TODO this is only needed by the fake client. Real K8s does not need it. Remove it once
		// the fake is fixed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.SchemeGroupVersion.String(),
				Kind:       "Deployment",
			},
		},
	},
		dl)
	if err != nil {
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) newGateway(ctx context.Context, namespace string) *v1.Deployment {
	labels := r.gatewayLabels()
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    namespace,
			GenerateName: "gcppubsub-channel-gateway",
			Labels:       labels,
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: r.gatewayServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: r.gatewayImage,
						},
					},
				},
			},
		},
	}
}
