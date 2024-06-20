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
	"strings"

	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	"knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"knative.dev/eventing/pkg/auth"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/network"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	rbacv1listers "k8s.io/client-go/listers/rbac/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"

	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/apis/feature"
	v1 "knative.dev/eventing/pkg/apis/messaging/v1"
	inmemorychannelreconciler "knative.dev/eventing/pkg/client/injection/reconciler/messaging/v1/inmemorychannel"
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/config"
	"knative.dev/eventing/pkg/reconciler/inmemorychannel/controller/resources"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process.
	dispatcherServiceAccountCreated = "DispatcherServiceAccountCreated"
	dispatcherRoleBindingCreated    = "DispatcherRoleBindingCreated"
	dispatcherDeploymentCreated     = "DispatcherDeploymentCreated"
	dispatcherServiceCreated        = "DispatcherServiceCreated"
	caCertsSecretKey                = eventingtls.SecretCACert
)

func newDeploymentWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherDeploymentFailed", "Reconciling dispatcher Deployment failed with: %w", err)
}

func newServiceWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherServiceFailed", "Reconciling dispatcher Service failed: %w", err)
}

func newServiceAccountWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherServiceAccountFailed", "Reconciling dispatcher ServiceAccount failed: %w", err)
}

func newRoleBindingWarn(err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "DispatcherRoleBindingFailed", "Reconciling dispatcher RoleBinding failed: %w", err)
}

type Reconciler struct {
	kubeClientSet kubernetes.Interface

	systemNamespace      string
	dispatcherImage      string
	deploymentLister     appsv1listers.DeploymentLister
	serviceLister        corev1listers.ServiceLister
	endpointsLister      corev1listers.EndpointsLister
	serviceAccountLister corev1listers.ServiceAccountLister
	secretLister         corev1listers.SecretLister
	roleBindingLister    rbacv1listers.RoleBindingLister

	eventDispatcherConfigStore *config.EventDispatcherConfigStore

	uriResolver *resolver.URIResolver

	eventPolicyLister v1alpha1.EventPolicyLister
}

// Check that our Reconciler implements Interface
var _ inmemorychannelreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, imc *v1.InMemoryChannel) pkgreconciler.Event {
	logging.FromContext(ctx).Infow("Reconciling", zap.Any("InMemoryChannel", imc))

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
		logging.FromContext(ctx).Errorw("Failed to reconcile InMemoryChannel dispatcher", zap.Error(err))
		return err
	}
	imc.Status.PropagateDispatcherStatus(&d.Status)

	// Make sure the dispatcher service exists and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	_, err = r.reconcileDispatcherService(ctx, scope, dispatcherNamespace, imc)
	if err != nil {
		logging.FromContext(ctx).Errorw("Failed to reconcile InMemoryChannel dispatcher service", zap.Error(err))
		return err
	}
	imc.Status.MarkServiceTrue()

	// Get the Dispatcher Service Endpoints and propagate the status to the Channel
	// endpoints has the same name as the service, so not a bug.
	e, err := r.endpointsLister.Endpoints(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			logging.FromContext(ctx).Error("Endpoints do not exist for dispatcher service")
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
		logging.FromContext(ctx).Errorw("Failed to reconcile channel service", zap.Error(err))
		return err
	}
	imc.Status.MarkChannelServiceTrue()

	// If a DeadLetterSink is defined in Spec.Delivery then whe resolve its URI and update the status
	if imc.Spec.Delivery != nil && imc.Spec.Delivery.DeadLetterSink != nil {
		deadLetterSinkAddr, err := r.uriResolver.AddressableFromDestinationV1(ctx, *imc.Spec.Delivery.DeadLetterSink, imc)
		if err != nil {
			logging.FromContext(ctx).Errorw("Unable to get the DeadLetterSink's URI", zap.Error(err))
			imc.Status.MarkDeadLetterSinkResolvedFailed("Unable to get the DeadLetterSink's URI", "%v", err)
			return fmt.Errorf("Failed to resolve Dead Letter Sink URI: %v", err)
		}
		imc.Status.MarkDeadLetterSinkResolvedSucceeded(eventingduck.NewDeliveryStatusFromAddressable(deadLetterSinkAddr))
	} else {
		imc.Status.MarkDeadLetterSinkNotConfigured()
	}

	featureFlags := feature.FromContext(ctx)
	if featureFlags.IsPermissiveTransportEncryption() {
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpAddress := r.httpAddress(svc)
		httpsAddress := r.httpsAddress(caCerts, imc)
		// Permissive mode:
		// - status.address http address with host-based routing
		// - status.addresses:
		//   - https address with path-based routing
		//   - http address with host-based routing
		imc.Status.Addresses = []duckv1.Addressable{httpsAddress, httpAddress}
		imc.Status.Address = &httpAddress
	} else if featureFlags.IsStrictTransportEncryption() {
		// Strict mode: (only https addresses)
		// - status.address https address with path-based routing
		// - status.addresses:
		//   - https address with path-based routing
		caCerts, err := r.getCaCerts()
		if err != nil {
			return err
		}

		httpsAddress := r.httpsAddress(caCerts, imc)
		imc.Status.Addresses = []duckv1.Addressable{httpsAddress}
		imc.Status.Address = &httpsAddress
	} else {
		httpAddress := r.httpAddress(svc)
		imc.Status.Address = &httpAddress
	}

	if featureFlags.IsOIDCAuthentication() {
		audience := auth.GetAudience(v1.SchemeGroupVersion.WithKind("InMemoryChannel"), imc.ObjectMeta)

		logging.FromContext(ctx).Debugw("Setting the imc audience", zap.String("audience", audience))
		imc.Status.Address.Audience = &audience
		for i := range imc.Status.Addresses {
			imc.Status.Addresses[i].Audience = &audience
		}
	} else {
		logging.FromContext(ctx).Debug("Clearing the imc audience as OIDC is not enabled")
		imc.Status.Address.Audience = nil
		for i := range imc.Status.Addresses {
			imc.Status.Addresses[i].Audience = nil
		}
	}

	imc.GetConditionSet().Manage(imc.GetStatus()).MarkTrue(v1.InMemoryChannelConditionAddressable)

	imc.Status.Policies = nil
	applyingEvenPolicies, err := auth.GetEventPoliciesForResource(r.eventPolicyLister, messagingv1.SchemeGroupVersion.WithKind("InMemoryChannel"), imc.ObjectMeta)
	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to get applying event policies for InMemoryChannel", zap.Error(err))
		imc.Status.MarkEventPoliciesFailed("EventPoliciesGetFailed", "Failed to get applying event policies")
	}

	if len(applyingEvenPolicies) > 0 {
		unreadyEventPolicies := []string{}
		for _, policy := range applyingEvenPolicies {
			if !policy.Status.IsReady() {
				unreadyEventPolicies = append(unreadyEventPolicies, policy.Name)
			} else {
				// only add Ready policies to the list
				imc.Status.Policies = append(imc.Status.Policies, eventingduck.AppliedEventPolicyRef{
					Name:       policy.Name,
					APIVersion: eventingv1alpha1.SchemeGroupVersion.String(),
				})
			}
		}

		if len(unreadyEventPolicies) == 0 {
			imc.Status.MarkEventPoliciesTrue()
		} else {
			imc.Status.MarkEventPoliciesFailed("EventPoliciesNotReady", "event policies %s are not ready", strings.Join(unreadyEventPolicies, ", "))
		}

	} else {
		// we have no applying event policy. So we set the EP condition to True
		if featureFlags.IsOIDCAuthentication() {
			// in case of OIDC auth, we also set the message with the default authorization mode
			imc.Status.MarkEventPoliciesTrueWithReason("DefaultAuthorizationMode", "Default authz mode is %q", featureFlags[feature.AuthorizationDefaultMode])
		} else {
			// in case OIDC is disabled, we set EP condition to true too, but give some message that authz (EPs) require OIDC
			imc.Status.MarkEventPoliciesTrueWithReason("OIDCDisabled", "Feature %q must be enabled to support Authorization", feature.OIDCAuthentication)
		}
	}

	// Ok, so now the Dispatcher Deployment & Service have been created, we're golden since the
	// dispatcher watches the Channel and where it needs to dispatch events to.
	logging.FromContext(ctx).Debugw("Reconciled InMemoryChannel", zap.Any("InMemoryChannel", imc))
	return nil
}

func (r *Reconciler) getCaCerts() (*string, error) {
	// Getting the secret called "imc-dispatcher-tls" from system namespace
	secret, err := r.secretLister.Secrets(r.systemNamespace).Get(eventingtls.IMCDispatcherServerTLSSecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to get CA certs from %s/%s: %w", r.systemNamespace, eventingtls.IMCDispatcherServerTLSSecretName, err)
	}
	caCerts, ok := secret.Data[caCertsSecretKey]
	if !ok {
		return nil, nil
	}
	return pointer.String(string(caCerts)), nil
}

func (r *Reconciler) httpAddress(svc *corev1.Service) duckv1.Addressable {
	// http address uses host-based routing
	httpAddress := duckv1.Addressable{
		Name: pointer.String("http"),
		URL:  apis.HTTP(network.GetServiceHostname(svc.Name, svc.Namespace)),
	}
	return httpAddress
}

func (r *Reconciler) httpsAddress(caCerts *string, imc *v1.InMemoryChannel) duckv1.Addressable {
	// https address uses path-based routing
	httpsAddress := duckv1.Addressable{
		Name:    pointer.String("https"),
		URL:     apis.HTTPS(fmt.Sprintf("%s.%s.svc.%s", dispatcherName, r.systemNamespace, network.GetClusterDomainName())),
		CACerts: caCerts,
	}
	httpsAddress.URL.Path = fmt.Sprintf("/%s/%s", imc.Namespace, imc.Name)
	return httpsAddress
}

func (r *Reconciler) reconcileDispatcher(ctx context.Context, scope, dispatcherNamespace string, imc *v1.InMemoryChannel) (*appsv1.Deployment, error) {
	if scope == eventing.ScopeNamespace {
		// Configure RBAC in namespace to access the configmaps
		// For cluster-deployed dispatcher, RBAC policies are already there.

		sa, err := r.reconcileServiceAccount(ctx, dispatcherNamespace, imc)
		if err != nil {
			return nil, err
		}

		if err := r.reconcileRoleBinding(ctx, dispatcherName, dispatcherNamespace, imc, dispatcherName, sa); err != nil {
			return nil, err
		}

		// Reconcile the RoleBinding allowing read access to the shared configmaps.
		// Note this RoleBinding is created in the system namespace and points to a
		// subject in the dispatcher's namespace.
		// TODO: might change when ConfigMapPropagation lands
		roleBindingName := fmt.Sprintf("%s-%s", dispatcherName, dispatcherNamespace)
		if err := r.reconcileRoleBinding(ctx, roleBindingName, r.systemNamespace, imc, "eventing-config-reader", sa); err != nil {
			return nil, err
		}
	}

	d, err := r.deploymentLister.Deployments(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			if scope == eventing.ScopeNamespace {
				// Create dispatcher in imc's namespace
				args := resources.DispatcherArgs{
					EventDispatcherConfig: config.EventDispatcherConfig{
						ConnectionArgs: r.eventDispatcherConfigStore.GetConfig().ConnectionArgs,
					},
					ServiceAccountName:  dispatcherName,
					DispatcherName:      dispatcherName,
					DispatcherNamespace: dispatcherNamespace,
					Image:               r.dispatcherImage,
				}
				expected := resources.MakeDispatcher(args)
				d, err := r.kubeClientSet.AppsV1().Deployments(dispatcherNamespace).Create(ctx, expected, metav1.CreateOptions{})
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

func (r *Reconciler) reconcileServiceAccount(ctx context.Context, dispatcherNamespace string, imc *v1.InMemoryChannel) (*corev1.ServiceAccount, error) {
	sa, err := r.serviceAccountLister.ServiceAccounts(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeServiceAccount(dispatcherNamespace, dispatcherName)
			sa, err := r.kubeClientSet.CoreV1().ServiceAccounts(dispatcherNamespace).Create(ctx, expected, metav1.CreateOptions{})
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

func (r *Reconciler) reconcileRoleBinding(ctx context.Context, name string, ns string, imc *v1.InMemoryChannel, clusterRoleName string, sa *corev1.ServiceAccount) error {
	_, err := r.roleBindingLister.RoleBindings(ns).Get(name)
	if err != nil {
		if apierrs.IsNotFound(err) {
			expected := resources.MakeRoleBinding(ns, name, sa, clusterRoleName)
			_, err := r.kubeClientSet.RbacV1().RoleBindings(ns).Create(ctx, expected, metav1.CreateOptions{})
			if err != nil {
				return newRoleBindingWarn(err)
			}
			controller.GetEventRecorder(ctx).Eventf(imc, corev1.EventTypeNormal, dispatcherRoleBindingCreated, "Dispatcher RoleBinding created")
			return nil
		}
		logging.FromContext(ctx).Error("Unable to get the dispatcher RoleBinding", zap.Error(err))
		imc.Status.MarkDispatcherFailed("DispatcherRoleBindingGetFailed", "Failed to get dispatcher RoleBinding")
		return newRoleBindingWarn(err)
	}
	return nil
}

func (r *Reconciler) reconcileDispatcherService(ctx context.Context, scope, dispatcherNamespace string, imc *v1.InMemoryChannel) (*corev1.Service, error) {
	svc, err := r.serviceLister.Services(dispatcherNamespace).Get(dispatcherName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			if scope == eventing.ScopeNamespace {
				expected := resources.MakeDispatcherService(dispatcherName, dispatcherNamespace)
				svc, err := r.kubeClientSet.CoreV1().Services(dispatcherNamespace).Create(ctx, expected, metav1.CreateOptions{})
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

func (r *Reconciler) reconcileChannelService(ctx context.Context, dispatcherNamespace string, imc *v1.InMemoryChannel) (*corev1.Service, error) {
	// Get the  Service and propagate the status to the Channel in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	expected, err := resources.NewK8sService(imc, resources.ExternalService(dispatcherNamespace, dispatcherName))
	if err != nil {
		logging.FromContext(ctx).Error("failed to create the channel service object", zap.Error(err))
		imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprint("Channel Service failed: ", err))
		return nil, err
	}

	channelSvcName := resources.CreateChannelServiceName(imc.Name)

	svc, err := r.serviceLister.Services(imc.Namespace).Get(channelSvcName)
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = r.kubeClientSet.CoreV1().Services(imc.Namespace).Create(ctx, expected, metav1.CreateOptions{})
			if err != nil {
				logging.FromContext(ctx).Error("failed to create the channel service", zap.Error(err))
				imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprint("Channel Service failed: ", err))
				return nil, err
			}
			return svc, nil
		}
		logging.FromContext(ctx).Error("Unable to get the channel service", zap.Error(err))
		imc.Status.MarkChannelServiceUnknown("ChannelServiceGetFailed", fmt.Sprint("Unable to get the channel service: ", err))
		return nil, err
	} else if !equality.Semantic.DeepEqual(svc.Spec, expected.Spec) {
		svc = svc.DeepCopy()
		svc.Spec = expected.Spec

		svc, err = r.kubeClientSet.CoreV1().Services(imc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			logging.FromContext(ctx).Error("failed to update the channel service", zap.Error(err))
			imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprint("Channel Service failed: ", err))
			return nil, err
		}
	}

	// Check to make sure that our IMC owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, imc) {
		err := fmt.Errorf("inmemorychannel: %s/%s does not own Service: %q", imc.Namespace, imc.Name, svc.Name)
		imc.Status.MarkChannelServiceFailed("ChannelServiceFailed", fmt.Sprint("Channel Service failed: ", err))
		return nil, err
	}
	return svc, nil
}
