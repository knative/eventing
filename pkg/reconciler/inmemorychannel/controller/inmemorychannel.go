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

package controller

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/resources"
	"knative.dev/eventing/pkg/utils"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	reconciled                      = "Reconciled"
	reconcileFailed                 = "ReconcileFailed"
	updateStatusFailed              = "UpdateStatusFailed"
	dispatcherServiceAccountCreated = "DispatcherServiceAccountCreated"
	dispatcherRoleBindingCreated    = "DispatcherRoleBindingCreated"
	dispatcherDeploymentCreated     = "DispatcherDeploymentCreated"
	dispatcherServiceCreated        = "DispatcherServiceCreated"
	observabilityConfigMapCreated   = "ObservabilityConfigMapCreated"
	tracingConfigMapCreated         = "TracingConfigMapCreated"

	scopeNamespace = "namespace"
	scopeCluster   = "cluster"
)

type Reconciler struct {
	*reconciler.Base

	systemNamespace         string
	dispatcherImage         string
	dispatcherScope         string
	inmemorychannelLister   listers.InMemoryChannelLister
	inmemorychannelInformer cache.SharedIndexInformer
	deploymentLister        appsv1listers.DeploymentLister
	serviceLister           corev1listers.ServiceLister
	endpointsLister         corev1listers.EndpointsLister
	serviceAccountLister    corev1listers.ServiceAccountLister
	roleBindingLister       rbacv1listers.RoleBindingLister
	impl                    *controller.Impl
}

var deploymentGVK = appsv1.SchemeGroupVersion.WithKind("Deployment")
var serviceGVK = corev1.SchemeGroupVersion.WithKind("Service")

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// Check that our Reconciler implements cache.ResourceEventHandler
var _ cache.ResourceEventHandler = (*Reconciler)(nil)

// ScopedFilter either filter with namespace and name for cluster-deployed dispatcher,
// and just by name for namespace-deployed dispatcher
func (r *Reconciler) ScopedFilter(namespace, name string) func(obj interface{}) bool {
	fnn := controller.FilterWithNameAndNamespace(namespace, name)
	fn := controller.FilterWithName(name)
	return func(obj interface{}) bool {
		if r.dispatcherScope == scopeCluster {
			return fnn(obj)
		}
		return fn(obj)
	}
}

// FilterWithNamespace makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on a namespace
func FilterWithNamespace(namespace string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if object, ok := obj.(metav1.Object); ok {
			return namespace == object.GetNamespace()
		}
		return false
	}
}

// cache.ResourceEventHandler implementation.

// For cluster-deployed dispatcher,
// these 3 functions just cause a Global Resync of the channels, because any changes here
// should be reflected onto the channels.

// For namespace-deployed dispatchers,
// these 3 functions only resync channel in the dispatcher namespace

func (r *Reconciler) OnAdd(obj interface{}) {
	r.impl.GlobalResync(r.inmemorychannelInformer)
}

func (r *Reconciler) OnUpdate(old, new interface{}) {
	r.impl.GlobalResync(r.inmemorychannelInformer)
}

func (r *Reconciler) OnDelete(obj interface{}) {
	r.impl.GlobalResync(r.inmemorychannelInformer)
}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the InMemoryChannel resource
// with the current status of the resource.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the InMemoryChannel resource with this namespace/name.
	original, err := r.inmemorychannelLister.InMemoryChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("InMemoryChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	channel := original.DeepCopy()

	// Reconcile this copy of the InMemoryChannel and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling InMemoryChannel", zap.Error(reconcileErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, reconcileFailed, "InMemoryChannel reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("InMemoryChannel reconciled")
		r.Recorder.Event(channel, corev1.EventTypeNormal, reconciled, "InMemoryChannel reconciled")
	}

	// Since the reconciler took a crack at this, make sure it's reflected
	// in the status correctly.
	channel.Status.ObservedGeneration = original.Generation

	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update InMemoryChannel status", zap.Error(updateStatusErr))
		r.Recorder.Eventf(channel, corev1.EventTypeWarning, updateStatusFailed, "Failed to update InMemoryChannel's status: %v", err)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, imc *v1alpha1.InMemoryChannel) error {
	imc.Status.InitializeConditions()

	if imc.DeletionTimestamp != nil {
		// Everything is cleaned up by the garbage collector.
		return nil
	}

	// We reconcile the status of the Channel by looking at:
	// 1. Dispatcher Deployment for it's readiness.
	// 2. Dispatcher k8s Service for it's existence.
	// 3. Dispatcher endpoints to ensure that there's something backing the Service.
	// 4. k8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service

	dispatcherNamespace := r.systemNamespace
	if r.dispatcherScope == scopeNamespace {
		dispatcherNamespace = imc.Namespace
	}

	// Make sure the dispatcher deployment exists and propagate the status to the Channel
	// For namespace-scope dispatcher, make sure configuration files exist and RBAC is properly configured.
	d, err := r.reconcileDispatcher(ctx, dispatcherNamespace, imc)
	if err != nil {
		return err
	}
	imc.Status.PropagateDispatcherStatus(&d.Status)

	// Make sure the dispatcher service exists and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.reconcileDispatcherService(ctx, dispatcherNamespace, imc)
	if err != nil {
		return err
	}
	imc.Status.MarkServiceTrue()

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			imc.Status.MarkEndpointsFailed("DispatcherEndpointsDoesNotExist", "Dispatcher Endpoints does not exist")
		} else {
			logging.FromContext(ctx).Error("Unable to get the dispatcher endpoints", zap.Error(err))
			imc.Status.MarkEndpointsUnknown("DispatcherEndpointsGetFailed", "Failed to get dispatcher endpoints")
		}
		return err
	}

	if len(e.Subsets) == 0 {
		logging.FromContext(ctx).Error("No endpoints found for Dispatcher service", zap.Error(err))
		imc.Status.MarkEndpointsFailed("DispatcherEndpointsNotReady", "There are no endpoints ready for Dispatcher service")
		return errors.New("there are no endpoints ready for Dispatcher service")
	}

	imc.Status.MarkEndpointsTrue()

	// Reconcile the k8s service representing the actual Channel. It points to the Dispatcher service via
	// ExternalName
	svc, err := r.reconcileChannelService(ctx, dispatcherNamespace, imc)
	if err != nil {
		return err
	}
	imc.Status.MarkChannelServiceTrue()
	imc.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, utils.GetClusterDomainName()),
	})

	if subscribableStatus := r.createSubscribableStatus(imc.Spec.Subscribable); subscribableStatus != nil {
		imc.Status.SubscribableTypeStatus.SetSubscribableTypeStatus(*subscribableStatus)
	}

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	return nil
}

func (r *Reconciler) reconcileDispatcher(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*appsv1.Deployment, error) {
	if r.dispatcherScope == scopeNamespace {
		// Configure RBAC in namespace to access the configmaps
		// For cluster-deployed dispatcher, RBAC policies are already there.

		sa, err := r.reconcileServiceAccount(ctx, dispatcherNamespace, imc)
		if err != nil {
			return nil, err
		}

		_, err = r.reconcileRoleBinding(ctx, dispatcherName, dispatcherNamespace, imc, dispatcherName, sa)
		if err != nil {
			return nil, err
		}

		// Reconcile the RoleBinding allowing read access to the shared configmaps.
		// Note this RoleBinding is created in the system namespace and points to a
		// subject in the dispatcher's namespace.
		// TODO: might change when ConfigMapPropagation lands
		roleBindingName := fmt.Sprintf("%s-%s", dispatcherName, dispatcherNamespace)
		_, err = r.reconcileRoleBinding(ctx, roleBindingName, r.systemNamespace, imc, "eventing-config-reader", sa)
		if err != nil {
			return nil, err
		}
	}

	d, err := r.deploymentLister.Deployments(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			if r.dispatcherScope == scopeNamespace {
				// Create dispatcher in imc's namespace
				args := resources.DispatcherArgs{
					ServiceAccountName:  dispatcherName,
					DispatcherName:      dispatcherName,
					DispatcherNamespace: dispatcherNamespace,
					Image:               r.dispatcherImage,
				}
				expected := resources.MakeDispatcher(args)
				d, err := r.KubeClientSet.AppsV1().Deployments(dispatcherNamespace).Create(expected)
				msg := "Dispatcher Deployment created"
				if err != nil {
					msg = fmt.Sprintf("not created, error: %v", err)
				}
				r.Recorder.Eventf(imc, corev1.EventTypeNormal, dispatcherDeploymentCreated, "%s", msg)
				return d, err
			}

			imc.Status.MarkDispatcherFailed("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")
		} else {
			logging.FromContext(ctx).Error("Unable to get the dispatcher Deployment", zap.Error(err))
			imc.Status.MarkDispatcherFailed("DispatcherDeploymentGetFailed", "Failed to get dispatcher Deployment")
		}
		return nil, err
	}
	return d, err
}

func (r *Reconciler) reconcileServiceAccount(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.ServiceAccount, error) {
	sa, err := r.serviceAccountLister.ServiceAccounts(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeServiceAccount(dispatcherNamespace, dispatcherName)
			sa, err := r.KubeClientSet.CoreV1().ServiceAccounts(dispatcherNamespace).Create(expected)
			msg := "Dispatcher ServiceAccount created"
			if err != nil {
				msg = fmt.Sprintf("Dispatcher ServiceAccount not created, error: %v", err)
			}
			r.Recorder.Eventf(imc, corev1.EventTypeNormal, dispatcherServiceAccountCreated, "%s", msg)
			return sa, err
		}

		logging.FromContext(ctx).Error("Unable to get the dispatcher ServiceAccount", zap.Error(err))
		imc.Status.MarkDispatcherFailed("DispatcherServiceAccountGetFailed", "Failed to get dispatcher ServiceAccount")
		return nil, err
	}
	return sa, err
}

func (r *Reconciler) reconcileRoleBinding(ctx context.Context, name string, ns string, imc *v1alpha1.InMemoryChannel, clusterRoleName string, sa *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	rb, err := r.roleBindingLister.RoleBindings(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeRoleBinding(ns, name, sa, clusterRoleName)
			rb, err := r.KubeClientSet.RbacV1().RoleBindings(ns).Create(expected)
			msg := "Dispatcher RoleBinding created"
			if err != nil {
				msg = fmt.Sprintf("Dispatcher RoleBinding not created, error: %v", err)
			}
			r.Recorder.Eventf(imc, corev1.EventTypeNormal, dispatcherRoleBindingCreated, "%s", msg)
			return rb, err
		}
		logging.FromContext(ctx).Error("Unable to get the dispatcher RoleBinding", zap.Error(err))
		imc.Status.MarkDispatcherFailed("DispatcherRoleBindingGetFailed", "Failed to get dispatcher RoleBinding")
		return nil, err
	}
	return rb, err
}

func (r *Reconciler) reconcileDispatcherService(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.Service, error) {
	svc, err := r.serviceLister.Services(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			if r.dispatcherScope == scopeNamespace {
				expected := resources.MakeDispatcherService(dispatcherName, dispatcherNamespace)
				svc, err := r.KubeClientSet.CoreV1().Services(dispatcherNamespace).Create(expected)
				msg := "Dispatcher Service created"
				if err != nil {
					msg = fmt.Sprintf("Dispatcher Service not created, error: %v", err)
				}
				r.Recorder.Eventf(imc, corev1.EventTypeNormal, dispatcherServiceCreated, "%s", msg)
				return svc, err
			}

			imc.Status.MarkServiceFailed("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")
		} else {
			logging.FromContext(ctx).Error("Unable to get the dispatcher service", zap.Error(err))
			imc.Status.MarkServiceFailed("DispatcherServiceGetFailed", "Failed to get dispatcher service")
		}
		return nil, err
	}
	return svc, err
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.Service, error) {
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.serviceLister.Services(imc.Namespace).Get(resources.CreateChannelServiceName(imc.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.NewK8sService(imc, resources.ExternalService(dispatcherNamespace, dispatcherName))
			if err != nil {
				logging.FromContext(ctx).Error("failed to create the channel service object", zap.Error(err))
				imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
				return nil, err
			}
			svc, err = r.KubeClientSet.CoreV1().Services(imc.Namespace).Create(svc)
			if err != nil {
				logging.FromContext(ctx).Error("failed to create the channel service", zap.Error(err))
				imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
				return nil, err
			}
			return svc, nil
		}
		logging.FromContext(ctx).Error("Unable to get the channel service", zap.Error(err))
		imc.Status.MarkChannelServiceUnknown("ChannelServiceGetFailed", fmt.Sprintf("Unable to get the channel service: %s", err))
		return nil, err
	}

	// Check to make sure that our IMC owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, imc) {
		err := fmt.Errorf("inmemorychannel: %s/%s does not own Service: %q", imc.Namespace, imc.Name, svc.Name)
		imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
		return nil, err
	}
	return svc, nil
}

func (r *Reconciler) createSubscribableStatus(subscribable *eventingduck.Subscribable) *eventingduck.SubscribableStatus {
	if subscribable == nil {
		return nil
	}
	subscriberStatus := make([]eventingduck.SubscriberStatus, 0)
	for _, sub := range subscribable.Subscribers {
		status := eventingduck.SubscriberStatus{
			UID:                sub.UID,
			ObservedGeneration: sub.Generation,
			Ready:              corev1.ConditionTrue,
		}
		subscriberStatus = append(subscriberStatus, status)
	}
	return &eventingduck.SubscribableStatus{
		Subscribers: subscriberStatus,
	}
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.InMemoryChannel) (*v1alpha1.InMemoryChannel, error) {
	imc, err := r.inmemorychannelLister.InMemoryChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(imc.Status, desired.Status) {
		return imc, nil
	}

	becomesReady := desired.Status.IsReady() && !imc.Status.IsReady()

	// Don't modify the informers copy.
	existing := imc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.EventingClientSet.MessagingV1alpha1().InMemoryChannels(desired.Namespace).UpdateStatus(existing)
	if err == nil && becomesReady {
		duration := time.Since(new.ObjectMeta.CreationTimestamp.Time)
		r.Logger.Infof("InMemoryChannel %q became ready after %v", imc.Name, duration)
		r.StatsReporter.ReportReady("InMemoryChannel", imc.Namespace, imc.Name, duration)
	}

	return new, err
}
