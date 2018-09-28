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

package clusterprovisioner

import (
	"context"
	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/buses/eventing/stub/channel"
	"github.com/knative/eventing/pkg/controller"
	"github.com/knative/eventing/pkg/sidecar/multichannelfanout"
	"github.com/knative/eventing/pkg/system"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"strings"
	"sync"
)

type reconciler struct {
	mgr    manager.Manager
	client     client.Client
	restConfig *rest.Config
	recorder   record.EventRecorder

	channelControllersLock sync.Mutex
	channelControllers map[corev1.ObjectReference]*channel.ConfigAndStopCh

	swapHttpHandlerConfig func(config multichannelfanout.Config) error

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

	cp := &eventingv1alpha1.ClusterProvisioner{}
	err := r.client.Get(ctx, request.NamespacedName, cp)

	// The ClusterProvisioner may have been deleted since it was added to the workqueue. If so,
	// there is nothing to be done.
	if errors.IsNotFound(err) {
		logger.Info("Could not find ClusterProvisioner", zap.Error(err))
		return reconcile.Result{}, nil
	}

	// Any other error should be retried in another reconciliation.
	if err != nil {
		logger.Error("Unable to Get ClusterProvisioner", zap.Error(err))
		return reconcile.Result{}, err
	}

	// Does this Controller control this ClusterProvisioner?
	if !shouldReconcile(cp) {
		logger.Info("Not reconciling ClusterProvisioner, it is not controlled by this Controller", zap.String("APIVersion", cp.APIVersion), zap.String("Kind", cp.Kind), zap.String("Namespace", cp.Namespace), zap.String("name", cp.Name))
		return reconcile.Result{}, nil
	}

	err = r.reconcile(ctx, cp)
	if err != nil {
		logger.Info("Error reconciling ClusterProvisioner", zap.Error(err))
		// Note that we do not return the error here, because we want to update the Status
		// regardless of the error.
	}

	updateStatusErr := r.updateClusterProvisionerStatus(ctx, cp)
	if updateStatusErr != nil {
		logger.Info("Error updating ClusterProvisioner Status", zap.Error(updateStatusErr))
		return reconcile.Result{}, updateStatusErr
	}

	return reconcile.Result{}, err
}

// shouldReconcile determines if this Controller should control (and therefore reconcile) a given
// ClusterProvisioner. This Controller only handles Stub buses.
func shouldReconcile(cp *eventingv1alpha1.ClusterProvisioner) bool {
	return cp.Namespace == "" && strings.HasPrefix(cp.Name, "stub-bus-provisioner-dispatcher")
}

func (r *reconciler) reconcile(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	logger := r.logger.With(zap.Any("clusterProvisioner", cp))

	// We are syncing two things:
	// 1. The K8s Service to talk to this Stub bus.
	//     - There is a single K8s Service for all requests going to this Stub bus.
	// 2. The in-memory Controller watching for Channels using this ClusterProvisioner.

	if cp.DeletionTimestamp != nil {
		// Delete the in-memory controller for the channels.
		// Delete the K8s service.
		err := r.deleteProvisioner(ctx, cp)
		if err != nil {
			logger.Info("Error deleting the Provisioner Controller for the ClusterProvisioner.", zap.Error(err))
			return err
		}
		err = r.deleteDispatcherService(ctx, cp)
		if err != nil {
			logger.Info("Error deleting the ClusterProvisioner's K8s Service", zap.Error(err))
			return err
		}
		// TODO: Delete the ClusterProvisioner? Remove a finalizer?
		return nil
	}

	err := r.createDispatcherService(ctx, cp)
	if err != nil {
		logger.Info("Error creating the ClusterProvisioner's K8s Service", zap.Error(err))
		return err
	}

	err = r.syncProvisioner(ctx, cp)
	if err != nil {
		logger.Info("Error creating the Provisioner Controller for the ClusterProvisioner", zap.Error(err))
		return err
	}

	r.setStatusReady(cp)
	return nil
}

func (r *reconciler) createDispatcherService(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	svcName := controller.ClusterBusDispatcherServiceName(cp.Name)
	svcKey := types.NamespacedName{
		Namespace: system.Namespace,
		Name:      svcName,
	}
	svc := &corev1.Service{}
	err := r.client.Get(ctx, svcKey, svc)

	if errors.IsNotFound(err) {
		svc = newDispatcherService(cp)
		err = r.client.Create(ctx, svc)
	}

	// If an error occurred in either Get or Create, we need to reconcile again.
	if err != nil {
		return err
	}

	// Ensure this ClusterProvisioner is the owner of the K8s service.
	if !metav1.IsControlledBy(svc, cp) {
		r.logger.Warn("ClusterProvisioner's K8s Service is not owned by the ClusterProvisioner", zap.Any("clusterProvisioner", cp), zap.Any("service", svc))
	}
	return nil
}

func (r *reconciler) deleteDispatcherService(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	// TODO: Rely on the garbage collector?
	return nil
}

func (r *reconciler) syncProvisioner(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	cpRef := objectRef(cp)

	cas, err := func() (*channel.ConfigAndStopCh, error) {
		r.channelControllersLock.Lock()
		defer r.channelControllersLock.Unlock()

		if _, ok := r.channelControllers[cpRef]; !ok {
			cas, err := channel.ProvideController(r.mgr, r.logger, &cpRef, r.syncAllChannelsToHttpHandler)
			if err != nil {
				return nil, err
			}
			r.channelControllers[cpRef] = cas
			return cas, nil
		} else {
			return nil, nil
		}
	}()
	if err != nil {
		return err
	}
	if cas != nil {
		cas.BackgroundStart()
		r.logger.Info("Starting Channel Controller", zap.Any("clusterProvisioner", cp))
	}
	return nil
}

func (r *reconciler) deleteProvisioner(ctx context.Context, cp *eventingv1alpha1.ClusterProvisioner) error {
	cpRef := objectRef(cp)

	// Keep the lock for as little time as possible.
	stopCh := func () chan<- struct{} {
		r.channelControllersLock.Lock()
		defer r.channelControllersLock.Unlock()

		if cc, ok := r.channelControllers[cpRef]; ok {
			delete(r.channelControllers, cpRef)
			return cc.StopCh
		} else {
			return nil
		}
	}()
	if stopCh != nil {
		stopCh <- struct{}{}
		r.logger.Info("Stopping Channel Controller", zap.Any("clusterProvisioner", cp))
	}
	return nil
}

func (r *reconciler) syncAllChannelsToHttpHandler() error {
	// Do in a function to hold the lock for as short a time as possible.
	configs := func() []multichannelfanout.Config {
		configs := make([]multichannelfanout.Config, 0)
		r.channelControllersLock.Lock()
		defer r.channelControllersLock.Unlock()
		for _, cas := range r.channelControllers {
			configs = append(configs, cas.Config())
		}
		return configs
	}()

	channelConfigs := make([]multichannelfanout.ChannelConfig, 0)
	for _, config := range configs {
		channelConfigs = append(channelConfigs, config.ChannelConfigs...)
	}
	combined := multichannelfanout.Config{
		ChannelConfigs: channelConfigs,
	}
	return r.swapHttpHandlerConfig(combined)
}

func objectRef(cp *eventingv1alpha1.ClusterProvisioner) corev1.ObjectReference {
	return corev1.ObjectReference{
		Namespace: cp.Namespace,
		Name: cp.Name,
		APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
		Kind: "ClusterProvisioner",
	}
}

func (r *reconciler) setStatusReady(cp *eventingv1alpha1.ClusterProvisioner) {
	cp.Status.Conditions = []duckv1alpha1.Condition{
		{
			Type:   duckv1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
		},
	}
}
func (r *reconciler) updateClusterProvisionerStatus(ctx context.Context, u *eventingv1alpha1.ClusterProvisioner) error {
	o := &eventingv1alpha1.ClusterProvisioner{}
	err := r.client.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, o)
	if err != nil {
		r.logger.Info("Error getting ClusterProvisioner for status update", zap.Error(err), zap.Any("updatedClusterProvisioner", u))
		return err
	}

	if !equality.Semantic.DeepEqual(o.Status, u.Status) {
		o.Status = u.Status
		return r.client.Update(ctx, o)
	}
	return nil
}

// newDispatcherService creates a new Service for a ClusterBus resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the ClusterBus resource that 'owns' it.
func newDispatcherService(cp *eventingv1alpha1.ClusterProvisioner) *corev1.Service {
	labels := dispatcherLabels(cp.Name)
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controller.ClusterBusDispatcherServiceName(cp.Name),
			Namespace: system.Namespace,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cp, schema.GroupVersionKind{
					Group:   eventingv1alpha1.SchemeGroupVersion.Group,
					Version: eventingv1alpha1.SchemeGroupVersion.Version,
					Kind:    "ClusterProvisioner",
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
}

func dispatcherLabels(cpName string) map[string]string {
	return map[string]string{
		"clusterProvisioner": cpName,
		"role":               "controller-dispatcher",
	}
}

