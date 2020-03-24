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

	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/controller"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/apis"
	pkgreconciler "knative.dev/pkg/reconciler"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	inmemorychannelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1alpha1/inmemorychannel"
	listers "knative.dev/eventing/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/resources"
	"knative.dev/eventing/pkg/utils"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	dispatcherServiceAccountCreated = "DispatcherServiceAccountCreated"
	dispatcherRoleBindingCreated    = "DispatcherRoleBindingCreated"
	dispatcherDeploymentCreated     = "DispatcherDeploymentCreated"
	dispatcherServiceCreated        = "DispatcherServiceCreated"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason InMemoryChannelReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "InMemoryChannelReconciled", "InMemoryChannel reconciled: \"%s/%s\"", namespace, name)
}

func newDeploymentWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherDeploymentFailed", "Reconciling dispatcher Deployment failed with: %s", err)
}

func newServiceWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherServiceFailed", "Reconciling dispatcher Service failed: %s", err)
}

func newServiceAccountWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherServiceAccountFailed", "Reconciling dispatcher ServiceAccount failed: %s", err)
}

func newRoleBindingWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherRoleBindingFailed", "Reconciling dispatcher RoleBinding failed: %s", err)
}

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	systemNamespace         string
	dispatcherImage         string
	inmemorychannelLister   listers.InMemoryChannelLister
	inmemorychannelInformer cache.SharedIndexInformer
	deploymentLister        appsv1listers.DeploymentLister
	serviceLister           corev1listers.ServiceLister
	endpointsLister         corev1listers.EndpointsLister
	serviceAccountLister    corev1listers.ServiceAccountLister
	roleBindingLister       rbacv1listers.RoleBindingLister
}

// Check that our Reconciler implements Interface
var _ inmemorychannelreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1alpha1.InMemoryChannel) pkgreconciler.Event {
	imc.Status.InitializeConditions()
	imc.Status.ObservedGeneration = imc.Generation

	// We reconcile the status of the Channel by looking at:
	// 1. Dispatcher Deployment for it's readiness.
	// 2. Dispatcher k8s Service for it's existence.
	// 3. Dispatcher endpoints to ensure that there's something backing the Service.
	// 4. k8s service representing the channel that will use ExternalName to point to the Dispatcher k8s service

	scope, ok := imc.Annotations[eventing.ScopeAnnotationKey]
	if !ok {
		scope = eventing.ScopeCluster
	}

	dispatcherNamespace := r.systemNamespace
	if scope == eventing.ScopeNamespace {
		dispatcherNamespace = imc.Namespace
	}

	// Make sure the dispatcher deployment exists and propagate the status to the Channel
	// For namespace-scope dispatcher, make sure configuration files exist and RBAC is properly configured.
	d, err := r.reconcileDispatcher(ctx, scope, dispatcherNamespace, imc)
	if err != nil {
		return err
	}
	imc.Status.PropagateDispatcherStatus(&d.Status)

	// Make sure the dispatcher service exists and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.reconcileDispatcherService(ctx, scope, dispatcherNamespace, imc)
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
	return newReconciledNormal(imc.Namespace, imc.Name)
}

func (r *Reconciler) reconcileDispatcher(ctx context.Context, scope, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*appsv1.Deployment, error) {
	if scope == eventing.ScopeNamespace {
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
			if scope == eventing.ScopeNamespace {
				// Create dispatcher in imc's namespace
				args := resources.DispatcherArgs{
					ServiceAccountName:  dispatcherName,
					DispatcherName:      dispatcherName,
					DispatcherNamespace: dispatcherNamespace,
					Image:               r.dispatcherImage,
				}
				expected := resources.MakeDispatcher(args)
				d, err := r.kubeClientSet.AppsV1().Deployments(dispatcherNamespace).Create(expected)
				if err != nil {
					return d, newDeploymentWarn(err)
				}
				controller.GetEventRecorder(ctx).Eventf(imc, corev1.EventTypeNormal, dispatcherDeploymentCreated, "Dispatcher Deployment created")
				return d, nil
			}

			imc.Status.MarkDispatcherFailed("DispatcherDeploymentDoesNotExist", "Dispatcher Deployment does not exist")
		} else {
			logging.FromContext(ctx).Error("Unable to get the dispatcher Deployment", zap.Error(err))
			imc.Status.MarkDispatcherFailed("DispatcherDeploymentGetFailed", "Failed to get dispatcher Deployment")
		}
		return nil, newDeploymentWarn(err)
	}
	return d, nil
}

func (r *Reconciler) reconcileServiceAccount(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.ServiceAccount, error) {
	sa, err := r.serviceAccountLister.ServiceAccounts(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeServiceAccount(dispatcherNamespace, dispatcherName)
			sa, err := r.kubeClientSet.CoreV1().ServiceAccounts(dispatcherNamespace).Create(expected)
			if err != nil {
				return sa, newServiceAccountWarn(err)
			}
			controller.GetEventRecorder(ctx).Eventf(imc, corev1.EventTypeNormal, dispatcherServiceAccountCreated, "Dispatcher ServiceAccount created")
			return sa, nil
		}

		logging.FromContext(ctx).Error("Unable to get the dispatcher ServiceAccount", zap.Error(err))
		imc.Status.MarkDispatcherFailed("DispatcherServiceAccountGetFailed", "Failed to get dispatcher ServiceAccount")
		return nil, newServiceAccountWarn(err)
	}
	return sa, nil
}

func (r *Reconciler) reconcileRoleBinding(ctx context.Context, name string, ns string, imc *v1alpha1.InMemoryChannel, clusterRoleName string, sa *corev1.ServiceAccount) (*rbacv1.RoleBinding, error) {
	rb, err := r.roleBindingLister.RoleBindings(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeRoleBinding(ns, name, sa, clusterRoleName)
			rb, err := r.kubeClientSet.RbacV1().RoleBindings(ns).Create(expected)
			if err != nil {
				return rb, newRoleBindingWarn(err)
			}
			controller.GetEventRecorder(ctx).Eventf(imc, corev1.EventTypeNormal, dispatcherRoleBindingCreated, "Dispatcher RoleBinding created")
			return rb, nil
		}
		logging.FromContext(ctx).Error("Unable to get the dispatcher RoleBinding", zap.Error(err))
		imc.Status.MarkDispatcherFailed("DispatcherRoleBindingGetFailed", "Failed to get dispatcher RoleBinding")
		return nil, newRoleBindingWarn(err)
	}
	return rb, nil
}

func (r *Reconciler) reconcileDispatcherService(ctx context.Context, scope, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.Service, error) {
	svc, err := r.serviceLister.Services(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			if scope == eventing.ScopeNamespace {
				expected := resources.MakeDispatcherService(dispatcherName, dispatcherNamespace)
				svc, err := r.kubeClientSet.CoreV1().Services(dispatcherNamespace).Create(expected)
				if err != nil {
					return svc, newServiceWarn(err)
				}
				controller.GetEventRecorder(ctx).Eventf(imc, corev1.EventTypeNormal, dispatcherServiceCreated, "Dispatcher Service created")
				return svc, nil
			}

			imc.Status.MarkServiceFailed("DispatcherServiceDoesNotExist", "Dispatcher Service does not exist")
		} else {
			logging.FromContext(ctx).Error("Unable to get the dispatcher service", zap.Error(err))
			imc.Status.MarkServiceFailed("DispatcherServiceGetFailed", "Failed to get dispatcher service")
		}
		return nil, newServiceWarn(err)
	}
	return svc, nil
}

func (r *Reconciler) reconcileChannelService(ctx context.Context, dispatcherNamespace string, imc *v1alpha1.InMemoryChannel) (*corev1.Service, error) {
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	expected, err := resources.NewK8sService(imc, resources.ExternalService(dispatcherNamespace, dispatcherName))
	if err != nil {
		logging.FromContext(ctx).Error("failed to create the channel service object", zap.Error(err))
		imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
		return nil, err
	}

	channelSvcName := resources.CreateChannelServiceName(imc.Name)

	svc, err := r.serviceLister.Services(imc.Namespace).Get(channelSvcName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = r.kubeClientSet.CoreV1().Services(imc.Namespace).Create(expected)
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
	} else if !equality.Semantic.DeepEqual(svc.Spec, expected.Spec) {
		svc = svc.DeepCopy()
		svc.Spec = expected.Spec

		svc, err = r.kubeClientSet.CoreV1().Services(imc.Namespace).Update(svc)
		if err != nil {
			logging.FromContext(ctx).Error("failed to update the channel service", zap.Error(err))
			imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprintf("Channel Service failed: %s", err))
			return nil, err
		}
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
