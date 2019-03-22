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

	"github.com/knative/eventing/pkg/logging"
	eventingreconciler "github.com/knative/eventing/pkg/reconciler"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	util "github.com/knative/eventing/pkg/provisioners"
	ccpcontroller "github.com/knative/eventing/pkg/provisioners/inmemory/clusterchannelprovisioner"
	"github.com/knative/eventing/pkg/reconciler/names"
	"github.com/knative/eventing/pkg/sidecar/configmap"
	"github.com/knative/eventing/pkg/sidecar/fanout"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
)

const (
	finalizerName = controllerAgentName
	// Name of the corev1.Events emitted from the reconciliation process
	channelConfigSyncFailed    = "ChannelConfigSyncFailed"
	k8sServiceCreateFailed     = "K8sServiceCreateFailed"
	virtualServiceCreateFailed = "VirtualServiceCreateFailed"
	// TODO after in-memory-channel is retired, asyncProvisionerName should be removed
	defaultProvisionerName = "in-memory-channel"
)

type reconciler struct {
	client       client.Client
	configMapKey client.ObjectKey
}

// Verify reconciler implements necessary interfaces
var _ eventingreconciler.EventingReconciler = &reconciler{}
var _ eventingreconciler.Finalizer = &reconciler{}

// eventingreconciler.EventingReconciler
func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

// eventingreconciler.EventingReconciler
func (r *reconciler) ReconcileResource(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) (bool, reconcile.Result, error) {
	// TODO: use this to store the logger and set a deadline
	c := obj.(*eventingv1alpha1.Channel)
	c.Status.InitializeConditions()
	logger := logging.FromContext(ctx)

	// We are syncing three things:
	// 1. The K8s Service to talk to this Channel.
	// 2. The Istio VirtualService to talk to this Channel.
	// 3. The configuration of all Channel subscriptions.

	// We always need to sync the Channel config, so do it first.
	if err := r.syncChannelConfig(ctx, logger); err != nil {
		logger.Info("Error syncing the Channel config", zap.Error(err))
		recorder.Eventf(c, corev1.EventTypeWarning, channelConfigSyncFailed, "Failed to sync Channel config: %v", err)
		return true, reconcile.Result{}, err
	}

	svc, err := util.CreateK8sService(ctx, r.client, c)
	if err != nil {
		logger.Info("Error creating the Channel's K8s Service", zap.Error(err))
		recorder.Eventf(c, corev1.EventTypeWarning, k8sServiceCreateFailed, "Failed to reconcile Channel's K8s Service: %v", err)
		return true, reconcile.Result{}, err
	}
	c.Status.SetAddress(names.ServiceHostName(svc.Name, svc.Namespace))

	if c.Spec.Provisioner.Name == defaultProvisionerName {
		_, err = util.CreateVirtualService(ctx, r.client, c, svc)
		if err != nil {
			logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
			recorder.Eventf(c, corev1.EventTypeWarning, virtualServiceCreateFailed, "Failed to reconcile Virtual Service for the Channel: %v", err)
			return true, reconcile.Result{}, err
		}
	} else {
		// We need to have a single dispatcher that is pointed at by _both_
		// ClusterChannelProvisioners. So fake the channel, by saying that its provisioner is the
		// one with the single dispatcher. The faked provisioner is used only to determine the
		// dispatcher Service's name.
		cCopy := c.DeepCopy()
		cCopy.Spec.Provisioner.Name = defaultProvisionerName
		_, err = util.CreateVirtualService(ctx, r.client, cCopy, svc)
		if err != nil {
			logger.Info("Error creating the Virtual Service for the Channel", zap.Error(err))
			recorder.Eventf(c, corev1.EventTypeWarning, virtualServiceCreateFailed, "Failed to reconcile Virtual Service for the Channel: %v", err)
			return true, reconcile.Result{}, err
		}
	}

	c.Status.MarkProvisioned()
	return true, reconcile.Result{}, nil
}

// eventingreconciler.EventingReconciler
func (r *reconciler) GetNewReconcileObject() eventingreconciler.ReconciledResource {
	return &v1alpha1.Channel{}
}

// eventingreconciler.Finalizer
func (r *reconciler) OnDelete(ctx context.Context, obj eventingreconciler.ReconciledResource, recorder record.EventRecorder) error {
	// Do not want to handle this error. It is better to panic here because this points to erroneous GetNewReconcileObject() function
	c := obj.(*v1alpha1.Channel)
	logger := logging.FromContext(ctx)
	if err := r.syncChannelConfig(ctx, logger); err != nil {
		logger.Info("Error syncing the Channel config", zap.Error(err))
		recorder.Eventf(c, corev1.EventTypeWarning, channelConfigSyncFailed, "Failed to sync Channel config: %v", err)
		return err
	}
	return nil
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterChannelProvisioner. This Controller only handles in-memory channels.
// eventingreconciler.EventingReconciler
func (r *reconciler) ShouldReconcile(_ context.Context, obj eventingreconciler.ReconciledResource, _ record.EventRecorder) bool {
	c := obj.(*eventingv1alpha1.Channel)
	if c.Spec.Provisioner != nil {
		return ccpcontroller.IsControlled(c.Spec.Provisioner)
	}
	return false
}

func (r *reconciler) syncChannelConfig(ctx context.Context, logger *zap.Logger) error {
	channels, err := r.listAllChannels(ctx)
	if err != nil {
		logger.Info("Unable to list channels", zap.Error(err))
		return err
	}
	config := multiChannelFanoutConfig(channels)
	return r.writeConfigMap(ctx, config, logger)
}

func (r *reconciler) writeConfigMap(ctx context.Context, config *multichannelfanout.Config, logger *zap.Logger) error {
	logger = logger.With(zap.Any("configMap", r.configMapKey))

	updated, err := configmap.SerializeConfig(*config)
	if err != nil {
		logger.Error("Unable to serialize config", zap.Error(err), zap.Any("config", config))
		return err
	}

	cm := &corev1.ConfigMap{}
	err = r.client.Get(ctx, r.configMapKey, cm)
	if errors.IsNotFound(err) {
		cm = r.createNewConfigMap(updated)
		err = r.client.Create(ctx, cm)
	}
	if err != nil {
		logger.Info("Unable to get/create ConfigMap", zap.Error(err))
		return err
	}

	if equality.Semantic.DeepEqual(cm.Data, updated) {
		// Nothing to update.
		return nil
	}

	cm.Data = updated
	return r.client.Update(ctx, cm)
}

func (r *reconciler) createNewConfigMap(data map[string]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: r.configMapKey.Namespace,
			Name:      r.configMapKey.Name,
		},
		Data: data,
	}
}

func multiChannelFanoutConfig(channels []eventingv1alpha1.Channel) *multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := multichannelfanout.ChannelConfig{
			Namespace: c.Namespace,
			Name:      c.Name,
		}
		if c.Spec.Subscribable != nil {
			// TODO After in-memory-channel is retired, this logic must be refactored.
			asyncHandler := false
			if c.Spec.Provisioner.Name != defaultProvisionerName {
				asyncHandler = true
			}
			channelConfig.FanoutConfig = fanout.Config{
				Subscriptions: c.Spec.Subscribable.Subscribers,
				AsyncHandler:  asyncHandler,
			}
		}
		cc = append(cc, channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}

func (r *reconciler) listAllChannels(ctx context.Context) ([]eventingv1alpha1.Channel, error) {
	channels := make([]eventingv1alpha1.Channel, 0)

	opts := &client.ListOptions{
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		cl := &eventingv1alpha1.ChannelList{}
		if err := r.client.List(ctx, opts, cl); err != nil {
			return nil, err
		}

		for _, c := range cl.Items {
			if r.ShouldReconcile(ctx, &c, nil) {
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
