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
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/system"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	istiov1alpha3 "github.com/knative/pkg/apis/istio/v1alpha3"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	PortName = "http"
	PortNumber = 80
)

type reconciler struct {
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	// This reconciler controls Channels provisioned by this stubProvisioner.
	stubProvisioner *corev1.ObjectReference

	config multichannelfanout.Config

	syncHttpChannelConfig func() error

	logger *zap.Logger
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &reconciler{}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	//TODO use this to store the logger and set a deadline
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
		logger.Info("Not reconciling Channel, it is not controlled by this Controller", zap.Any("stubProvisioner", r.stubProvisioner), zap.Any("ref", c.Spec.Provisioner.Ref))
		return reconcile.Result{}, nil
	}
	logger.Debug("Reconciling Channel")

	err = r.reconcile(ctx, c)
	if err != nil {
		logger.Info("Error reconciling Channel", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	updateStatusErr := r.updateChannelStatus(ctx, c)
	if updateStatusErr != nil {
		logger.Info("Error updating Channel Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterProvisioner. This Controller only handles Stub buses.
func (r *reconciler) shouldReconcile(c *eventingv1alpha1.Channel) bool {
	return equality.Semantic.DeepEqual(r.stubProvisioner, c.Spec.Provisioner.Ref)
}

func (r *reconciler) reconcile(ctx context.Context, c *eventingv1alpha1.Channel) error {
	logger := r.logger.With(zap.Any("channel", c))

	// We are syncing three things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	// 3. The configuration of Channel's subscriptions.

	// We always need to sync the Channel config, so do it first.
	err := r.syncChannelConfig(ctx)
	if err != nil {
		logger.Info("Error removing the Channel's config", zap.Error(err))
		return err
	}

	if c.DeletionTimestamp != nil {
		err := r.deleteVirtualService(ctx, c)
		if err != nil {
			logger.Info("Error deleting the Provisioner Controller for the ClusterProvisioner.", zap.Error(err))
			return err
		}
		err = r.deleteK8sService(ctx, c)
		if err != nil {
			logger.Info("Error deleting the ClusterProvisioner's K8s Service", zap.Error(err))
			return err
		}
		return nil
	}

	err = r.syncChannelConfig(ctx)
	if err != nil {
		logger.Info("Error syncing the Channel config", zap.Error(err))
		return err
	}
	err = r.createK8sService(ctx, c)
	if err != nil {
		logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		return err
	}

	err = r.createVirtualService(ctx, c)
	if err != nil {
		logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
		return err
	}

	r.setStatusReady(c)
	return nil
}

func (r *reconciler) getK8sService(ctx context.Context, c *eventingv1alpha1.Channel) (*corev1.Service, error) {
		svcKey := types.NamespacedName{
		Namespace: c.Namespace,
		Name:      controller.ChannelServiceName(c.Name),
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)
	return svc, err
}

func (r *reconciler) createK8sService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	svc, err := r.getK8sService(ctx, c)

	if errors.IsNotFound(err) {
		svc = newK8sService(c)
		err = r.client.Create(ctx, svc)
	}

	// If an error occurred in either Get or Create, we need to reconcile again.
	if err != nil {
		return err
	}

	// Ensure this Channel is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, c) {
		r.logger.Warn("Channel's K8s Service is not owned by the Channel", zap.Any("channel", c), zap.Any("service", svc))
	}
	return nil
}

func (r *reconciler) deleteK8sService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	// TODO: Rely on the garbage collector?
	svc, err := r.getK8sService(ctx, c)

	if errors.IsNotFound(err) {
		// Nothing to delete.
		return nil
	}

	if err != nil {
		return err
	}

	// Verify that we own the Service before deleting it.
	if metav1.IsControlledBy(svc, c) {
		return r.client.Delete(ctx, svc)
	} else {
		r.logger.Warn("Channel's K8s Service is not owned by the Channel. Not deleting it.", zap.Any("channel", c), zap.Any("service", svc))
		return nil
	}
}





func (r *reconciler) createVirtualService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	virtualService, err := r.getVirtualService(ctx, c)

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		virtualService = newVirtualService(c)
		err = r.client.Create(ctx, virtualService)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Virtual Service is not controlled by this Channel, we should log a warning, but don't
	// consider it an error.
	if !metav1.IsControlledBy(virtualService, c) {
		r.logger.Warn("VirtualService not owned by Channel", zap.Any("channel", c), zap.Any("virtualService", virtualService))
	}
	return nil
}

func (r *reconciler) deleteVirtualService(ctx context.Context, c *eventingv1alpha1.Channel) error {
	virtualService, err := r.getVirtualService(ctx, c)

	// If the resource doesn't exist, there is nothing to delete.
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	// Verify that we own the VirtualService before deleting it.
	if metav1.IsControlledBy(virtualService, c) {
		return r.client.Delete(ctx, virtualService)
	} else {
		r.logger.Warn("VirtualService exists, but not owned by Channel. Not deleting it.", zap.Any("channel", c), zap.Any("virtualService", virtualService))
		return nil
	}
}

func (r *reconciler) getVirtualService(ctx context.Context, c *eventingv1alpha1.Channel) (*istiov1alpha3.VirtualService, error) {
	vsk := client.ObjectKey{
		Namespace: c.Namespace,
		Name: controller.ChannelVirtualServiceName(c.ObjectMeta.Name),
	}
	vs := &istiov1alpha3.VirtualService{}
	err := r.client.Get(ctx, vsk, vs)
	return vs, err
}

// newService creates a new Service for a Channel resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Channel resource that 'owns' it.
func newK8sService(c *eventingv1alpha1.Channel) *corev1.Service {
	labels := map[string]string{
		"channel": c.Name,
		"provisioner": c.Spec.Provisioner.Ref.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ChannelServiceName(c.ObjectMeta.Name),
			Namespace: c.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(c, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: PortName,
					Port: PortNumber,
				},
			},
		},
	}
}

// newVirtualService creates a new VirtualService for a Channel resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Channel resource that 'owns' it.
func newVirtualService(channel *eventingv1alpha1.Channel) *istiov1alpha3.VirtualService {
	labels := map[string]string{
		"channel":     channel.Name,
		"provisioner": channel.Spec.Provisioner.Ref.Name,
	}
	destinationHost := controller.ServiceHostName(controller.ClusterBusDispatcherServiceName(channel.Spec.Provisioner.Ref.Name), system.Namespace)
	return &istiov1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ChannelVirtualServiceName(channel.Name),
			Namespace: channel.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(channel, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Channel",
				}),
			},
		},
		Spec: istiov1alpha3.VirtualServiceSpec{
			Hosts: []string{
				controller.ServiceHostName(controller.ChannelServiceName(channel.Name), channel.Namespace),
				controller.ChannelHostName(channel.Name, channel.Namespace),
			},
			Http: []istiov1alpha3.HTTPRoute{{
				Rewrite: &istiov1alpha3.HTTPRewrite{
					Authority: controller.ChannelHostName(channel.Name, channel.Namespace),
				},
				Route: []istiov1alpha3.DestinationWeight{{
					Destination: istiov1alpha3.Destination{
						Host: destinationHost,
						Port: istiov1alpha3.PortSelector{
							Number: PortNumber,
						},
					}},
				}},
			},
		},
	}
}

func (r *reconciler) setStatusReady(c *eventingv1alpha1.Channel) {
	c.Status.Conditions = []duckv1alpha1.Condition{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
		},
	}
}
func (r *reconciler) updateChannelStatus(ctx context.Context, u *eventingv1alpha1.Channel) error {
	o := &eventingv1alpha1.Channel{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o)
	if err != nil {
		r.logger.Info("Error getting Channel for status update", zap.Error(err), zap.Any("updatedChannel", u))
		return err
	}

	if !equality.Semantic.DeepEqual(o.Status, u.Status) {
		o.Status = u.Status
		return r.client.Update(ctx, o)
	}
	return nil
}

func (r *reconciler) syncChannelConfig(ctx context.Context) error {
	channels, err := r.listAllChannels(ctx)
	if err != nil {
		r.logger.Info("Unable to list channels", zap.Error(err))
		return err
	}
	config := multiChannelFanoutConfig(channels)
	r.config = config
	return r.syncHttpChannelConfig()
}

func (r *reconciler) getConfig() multichannelfanout.Config {
	return r.config
}

func multiChannelFanoutConfig(channels []eventingv1alpha1.Channel) multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
		channelable := c.Spec.Channelable
		if channelable != nil {
			cc = append(cc, multichannelfanout.ChannelConfig{
				Namespace: c.Namespace,
				Name:      c.Name,
				FanoutConfig: fanout.Config{
					Subscriptions: c.Spec.Channelable.Subscribers,
				},
			})
		}
	}
	return multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}

func (r *reconciler) listAllChannels(ctx context.Context) ([]eventingv1alpha1.Channel, error) {
	channels := make([]eventingv1alpha1.Channel, 0)

	opts := &client.ListOptions{}
	for {
		cl := &eventingv1alpha1.ChannelList{}
		err := r.client.List(ctx, opts, cl)
		if err != nil {
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
			return channels, err
		}
	}
}
