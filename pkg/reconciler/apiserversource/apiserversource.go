/*
Copyright 2020 The Knative Authors

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

package apiserversource

import (
	"context"
	"encoding/json"
	"fmt"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"sort"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"

	clientv1 "k8s.io/client-go/listers/core/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	"knative.dev/eventing/pkg/apis/feature"
	apisources "knative.dev/eventing/pkg/apis/sources"
	v1 "knative.dev/eventing/pkg/apis/sources/v1"
	"knative.dev/eventing/pkg/auth"
	apiserversourcereconciler "knative.dev/eventing/pkg/client/injection/reconciler/sources/v1/apiserversource"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	apiserversourceDeploymentCreated = "ApiServerSourceDeploymentCreated"
	apiserversourceDeploymentUpdated = "ApiServerSourceDeploymentUpdated"

	component = "apiserversource"
)

func newWarningSinkNotFound(sink *duckv1.Destination) pkgreconciler.Event {
	b, _ := json.Marshal(sink)
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkNotFound", "Sink not found: %s", string(b))
}

// Reconciler reconciles a ApiServerSource object
type Reconciler struct {
	kubeClientSet kubernetes.Interface

	receiveAdapterImage string

	ceSource     string
	sinkResolver *resolver.URIResolver

	configs         reconcilersource.ConfigAccessor
	namespaceLister clientv1.NamespaceLister

	serviceAccountLister clientv1.ServiceAccountLister
}

var _ apiserversourcereconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1.ApiServerSource) pkgreconciler.Event {
	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	// 3. Create the EventType that it can emit.
	//     - Will be garbage collected by K8s when this CronJobSource is deleted.
	dest := source.Spec.Sink.DeepCopy()
	if dest.Ref != nil {
		// To call URIFromDestination(), dest.Ref must have a Namespace. If there is
		// no Namespace defined in dest.Ref, we will use the Namespace of the source
		// as the Namespace of dest.Ref.
		if dest.Ref.Namespace == "" {
			//TODO how does this work with deprecated fields
			dest.Ref.Namespace = source.GetNamespace()
		}
	}

	// OIDC authentication
	featureFlags := feature.FromContext(ctx)
	if err := auth.SetupOIDCServiceAccount(ctx, featureFlags, r.serviceAccountLister, r.kubeClientSet, v1.SchemeGroupVersion.WithKind("ApiServerSource"), source.ObjectMeta, &source.Status, func(as *duckv1.AuthStatus) {
		source.Status.Auth = as
	}); err != nil {
		return err
	}

	if featureFlags.IsOIDCAuthentication() {
		// Create the role
		logging.FromContext(ctx).Errorw("haha: About to enter the role access granting stage Creating role and rolebinding")
		err := createOIDCRole(ctx, r.kubeClientSet, v1.SchemeGroupVersion.WithKind("ApiServerSource"), source.ObjectMeta)

		if err != nil {
			logging.FromContext(ctx).Errorw("Failed when creating the OIDC Role for ApiServerSource", zap.Error(err))
			return err
		}

		// Create the rolebinding

		err = createOIDCRoleBinding(ctx, r.kubeClientSet, v1.SchemeGroupVersion.WithKind("ApiServerSource"), source.ObjectMeta, source.Spec.ServiceAccountName)
		if err != nil {
			logging.FromContext(ctx).Errorw("Failed when creating the OIDC RoleBinding for ApiServerSource", zap.Error(err))
			return err
		}
	}




	logging.FromContext(ctx).Errorw("haha: finished About to enter the role access granting stage Creating role and rolebinding")
	sinkAddr, err := r.sinkResolver.AddressableFromDestinationV1(ctx, *dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return newWarningSinkNotFound(dest)
	}
	source.Status.MarkSink(sinkAddr)

	// resolve namespaces to watch
	namespaces, err := r.namespacesFromSelector(source)
	if err != nil {
		logging.FromContext(ctx).Errorw("cannot retrieve namespaces to watch", zap.Error(err))
		return err
	}
	source.Status.Namespaces = namespaces

	err = r.runAccessCheck(ctx, source, namespaces)
	if err != nil {
		logging.FromContext(ctx).Errorw("Not enough permission", zap.Error(err))
		return err
	}

	// An empty selector targets all namespaces.
	allNamespaces := isEmptySelector(source.Spec.NamespaceSelector)
	ra, err := r.createReceiveAdapter(ctx, source, sinkAddr, namespaces, allNamespaces)
	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to create the receive adapter", zap.Error(err))
		return err
	}

	logging.FromContext(ctx).Debugw("haha: Receive adapter created", zap.Any("receiveAdapter", ra))
	source.Status.PropagateDeploymentAvailability(ra)

	logging.FromContext(ctx).Debugw("haha: Creating CloudEventAttributes")

	cloudEventAttributes, err := r.createCloudEventAttributes(source)
	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to create CloudEventAttributes", zap.Error(err))
		return err
	}
	source.Status.CloudEventAttributes = cloudEventAttributes

	return nil
}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *v1.ApiServerSource) pkgreconciler.Event {
	logging.FromContext(ctx).Info("Deleting source")
	// Allow for eventtypes to be cleaned up
	source.Status.CloudEventAttributes = []duckv1.CloudEventAttributes{}
	return nil
}

func (r *Reconciler) namespacesFromSelector(src *v1.ApiServerSource) ([]string, error) {
	if src.Spec.NamespaceSelector == nil {
		return []string{src.Namespace}, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(src.Spec.NamespaceSelector)
	if err != nil {
		return nil, err
	}

	namespaces, err := r.namespaceLister.List(selector)
	if err != nil {
		return nil, err
	}

	nsString := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		nsString = append(nsString, ns.Name)
	}
	sort.Strings(nsString)
	return nsString, nil
}

func isEmptySelector(selector *metav1.LabelSelector) bool {
	if selector == nil {
		return false
	}

	if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 0 {
		return true
	}

	return false
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1.ApiServerSource, sinkAddr *duckv1.Addressable, namespaces []string, allNamespaces bool) (*appsv1.Deployment, error) {
	// TODO: missing.
	// if err := checkResourcesStatus(src); err != nil {
	// 	return nil, err
	// }

	// trying to figure out why audience is empty

	adapterArgs := resources.ReceiveAdapterArgs{
		Image:         r.receiveAdapterImage,
		Source:        src,
		Labels:        resources.Labels(src.Name),
		CACerts:       sinkAddr.CACerts,
		SinkURI:       sinkAddr.URL.String(),
		Audience:      sinkAddr.Audience,
		Configs:       r.configs,
		Namespaces:    namespaces,
		AllNamespaces: allNamespaces,
	}

	expected, err := resources.MakeReceiveAdapter(&adapterArgs)
	if err != nil {
		return nil, err
	}

	ra, err := r.kubeClientSet.AppsV1().Deployments(src.Namespace).Get(ctx, expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		ra, err = r.kubeClientSet.AppsV1().Deployments(src.Namespace).Create(ctx, expected, metav1.CreateOptions{})
		msg := "Deployment created"
		if err != nil {
			msg = fmt.Sprint("Deployment created, error:", err)
		}
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentCreated, "%s", msg)
		return ra, err
	} else if err != nil {
		return nil, fmt.Errorf("error getting receive adapter: %v", err)
	} else if !metav1.IsControlledBy(ra, src) {
		return nil, fmt.Errorf("deployment %q is not owned by ApiServerSource %q", ra.Name, src.Name)
	} else if r.podSpecChanged(ra.Spec.Template.Spec, expected.Spec.Template.Spec) {
		ra.Spec.Template.Spec = expected.Spec.Template.Spec
		if ra, err = r.kubeClientSet.AppsV1().Deployments(src.Namespace).Update(ctx, ra, metav1.UpdateOptions{}); err != nil {
			return ra, err
		}
		controller.GetEventRecorder(ctx).Eventf(src, corev1.EventTypeNormal, apiserversourceDeploymentUpdated, "Deployment %q updated", ra.Name)
		return ra, nil
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
	}
	return ra, nil
}

func (r *Reconciler) podSpecChanged(oldPodSpec corev1.PodSpec, newPodSpec corev1.PodSpec) bool {
	if !equality.Semantic.DeepDerivative(newPodSpec, oldPodSpec) {
		return true
	}
	if len(oldPodSpec.Containers) != len(newPodSpec.Containers) {
		return true
	}
	for i := range newPodSpec.Containers {
		if !equality.Semantic.DeepEqual(newPodSpec.Containers[i].Env, oldPodSpec.Containers[i].Env) {
			return true
		}
	}
	return false
}

func (r *Reconciler) runAccessCheck(ctx context.Context, src *v1.ApiServerSource, namespaces []string) error {
	if src.Spec.Resources == nil || len(src.Spec.Resources) == 0 {
		src.Status.MarkSufficientPermissions()
		return nil
	}

	// Run the basic service account access check (This is not OIDC service account)
	user := "system:serviceaccount:" + src.Namespace + ":"
	if src.Spec.ServiceAccountName == "" {
		logging.FromContext(ctx).Debugw("haha No ServiceAccountName specified, using default")
		user += "default"
	} else {
		logging.FromContext(ctx).Debugw("haha Using ServiceAccountName", zap.String("ServiceAccountName", src.Spec.ServiceAccountName))
		user += src.Spec.ServiceAccountName
	}

	verbs := []string{"get", "list", "watch"}
	lastReason := ""

	// Collect all missing permissions.
	missing := ""
	sep := ""

	for _, res := range src.Spec.Resources {
		gv, err := schema.ParseGroupVersion(res.APIVersion)
		if err != nil {
			return err
		}
		gvr, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: res.Kind, Group: gv.Group, Version: gv.Version}) // TODO: Test for nil Kind.

		for _, ns := range namespaces {
			missingVerbs := ""
			sep1 := ""
			for _, verb := range verbs {
				sar := &authorizationv1.SubjectAccessReview{
					Spec: authorizationv1.SubjectAccessReviewSpec{
						ResourceAttributes: &authorizationv1.ResourceAttributes{
							Namespace: ns,
							Verb:      verb,
							Group:     gv.Group,
							Resource:  gvr.Resource,
						},
						User: user,
					},
				}

				response, err := r.kubeClientSet.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
				if err != nil {
					return err
				}

				if !response.Status.Allowed {
					missingVerbs += sep1 + verb
					sep1 = ", "
				}
			}

			if missingVerbs != "" {
				missing += sep + missingVerbs + ` resource "` + gvr.Resource + `" in API group "` + gv.Group + `" in Namespace "` + ns + `"`
				sep = ", "
			}
		}
	}
	if missing == "" {
		src.Status.MarkSufficientPermissions()
		return nil
	}

	src.Status.MarkNoSufficientPermissions(lastReason, "User %s cannot %s", user, missing)
	return fmt.Errorf("insufficient permissions: User %s cannot %s", user, missing)

}

func (r *Reconciler) createCloudEventAttributes(src *v1.ApiServerSource) ([]duckv1.CloudEventAttributes, error) {
	var eventTypes []string
	if src.Spec.EventMode == v1.ReferenceMode {
		eventTypes = apisources.ApiServerSourceEventReferenceModeTypes
	} else if src.Spec.EventMode == v1.ResourceMode {
		eventTypes = apisources.ApiServerSourceEventResourceModeTypes
	} else {
		return []duckv1.CloudEventAttributes{}, fmt.Errorf("no EventType available for EventMode: %s", src.Spec.EventMode)
	}
	ceAttributes := make([]duckv1.CloudEventAttributes, 0, len(eventTypes))
	for _, apiServerSourceType := range eventTypes {
		ceAttributes = append(ceAttributes, duckv1.CloudEventAttributes{
			Type:   apiServerSourceType,
			Source: r.ceSource,
		})
	}
	return ceAttributes, nil
}

// TODO: adding the rolebinding function to the resource folder and the role to the auth folder

// createOIDCRole: this function will call resources package to get the role object
// and then pass to kubeclient to make the actual OIDC role
func createOIDCRole (ctx context.Context,kubeclient kubernetes.Interface , gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) (error){

	roleName := "create-oidc-token"
	// First, create the role object

	//Second, call kubeclient and see whether the role exist or not
	role, err := kubeclient.RbacV1().Roles(objectMeta.Namespace).Get(ctx, roleName, metav1.GetOptions{})
	logging.FromContext(ctx).Errorw("haha in the controller: role object %s", zap.Any("role",role))

	if apierrs.IsNotFound(err) {
		role,err := resources.MakeOIDCRole(ctx,gvk,objectMeta)
		logging.FromContext(ctx).Errorw("haha in the controller: not found", zap.Error(err))
		// If the role does not exist, we will call kubeclient to create it
		_, err = kubeclient.RbacV1().Roles(objectMeta.Namespace).Create(ctx, role, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("could not create OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
				}

	}

	return nil

}


// createOIDCRoleBinding:  this function will call resources package to get the rolebinding object
// and then pass to kubeclient to make the actual OIDC rolebinding
func createOIDCRoleBinding (ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta, saName string) (error) {
	roleBindingName := "create-oidc-token"
	// First, create the rolebinding object


	// Second, call kubeclient and see whether the role exist or not
	_, err := kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Get(ctx, roleBindingName, metav1.GetOptions{})

	if apierrs.IsNotFound(err) {
		roleBinding, err := resources.MakeOIDCRoleBinding(ctx, gvk, objectMeta, saName)
		// If the role does not exist, we will call kubeclient to create it
		_, err = kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account rolebinding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
		}

	}
	return nil
}






// EnsureOIDCServiceAccountRoleBindingExistsForResource
// makes sure the given resource has an OIDC service account role binding with
// an owner reference to the resource set.
//func EnsureOIDCServiceAccountRoleBindingExistsForResource(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta, saName string) error {
//	logger := logging.FromContext(ctx)
//	logger.Errorf("haha: Initializing")
//
//	roleName := fmt.Sprintf("create-oidc-token")
//	roleBindingName := fmt.Sprintf("create-oidc-token")
//
//	logger.Errorf("haha: going to get role binding for %s", objectMeta.Name)
//	logger.Errorf("haha: role name %s", roleName)
//	logger.Errorf("haha: role binding name %s", roleBindingName)
//
//	roleBinding, err := kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Get(ctx, roleBindingName, metav1.GetOptions{})
//
//	logger.Errorf("haha: got role binding for %s", roleBinding)
//
//	logger.Errorf("haha: going to enter the if statement")
//	// If the resource doesn't exist, we'll create it.
//	if apierrs.IsNotFound(err) {
//		logging.FromContext(ctx).Debugw("Creating OIDC service account role binding", zap.Error(err))
//		logger.Errorf("haha: creating role binding")
//
//		// Create the "create-oidc-token" role
//		CreateRoleForServiceAccount(ctx, kubeclient, gvk, objectMeta)
//
//		roleBinding = &rbacv1.RoleBinding{
//			ObjectMeta: metav1.ObjectMeta{
//				Name:      roleBindingName,
//				Namespace: objectMeta.GetNamespace(),
//				Annotations: map[string]string{
//					"description": fmt.Sprintf("Role Binding for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
//				},
//			},
//			RoleRef: rbacv1.RoleRef{
//				APIGroup: "rbac.authorization.k8s.io",
//				Kind:     "Role",
//				//objectMeta.Name + roleName
//				Name: fmt.Sprintf(roleName),
//			},
//			Subjects: []rbacv1.Subject{
//				{
//					Kind:      "ServiceAccount",
//					Namespace: objectMeta.GetNamespace(),
//					// apiServerSource service account name, it is in the source.Spec, NOT in source.Auth
//					Name: saName,
//				},
//			},
//		}
//
//		logger.Errorf("haha: role binding object created")
//		logger.Errorf("haha: role binding object %s", roleBinding)
//
//		_, err = kubeclient.RbacV1().RoleBindings(objectMeta.Namespace).Create(ctx, roleBinding, metav1.CreateOptions{})
//		if err != nil {
//			logger.Errorf("haha: error creating role binding")
//			logger.Errorf("haha: error %s", err)
//			return fmt.Errorf("could not create OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
//		}
//
//		return nil
//	}
//
//	if err != nil {
//		logger.Errorf("haha: error getting role binding")
//		logger.Errorf("haha: error %s", err)
//		return fmt.Errorf("could not get OIDC service account role binding %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
//
//	}
//
//	if !metav1.IsControlledBy(&roleBinding.ObjectMeta, &objectMeta) {
//		logger.Errorf("haha: role binding not owned by")
//		logger.Errorf("haha: role binding %s", roleBinding)
//		return fmt.Errorf("role binding %s not owned by %s %s", roleBinding.Name, gvk.Kind, objectMeta.Name)
//	}
//
//	return nil
//}
//
//// Create the create-oidc-token role for the service account
//func CreateRoleForServiceAccount(ctx context.Context, kubeclient kubernetes.Interface, gvk schema.GroupVersionKind, objectMeta metav1.ObjectMeta) error {
//
//	logger := logging.FromContext(ctx)
//	logger.Errorf("haha: Initializing create role for service account")
//
//	roleName := fmt.Sprintf("create-oidc-token")
//	logger.Errorf("haha: role name %s", roleName)
//
//	role, err := kubeclient.RbacV1().Roles(objectMeta.Namespace).Get(ctx, roleName, metav1.GetOptions{})
//	logger.Errorf("haha: got role %s", role)
//
//	// If the resource doesn't exist, we'll create it.
//	if apierrs.IsNotFound(err) {
//		logging.FromContext(ctx).Debugw("Creating OIDC service account role", zap.Error(err))
//
//		logger.Errorf("haha: creating role")
//
//		role = &rbacv1.Role{
//			ObjectMeta: metav1.ObjectMeta{
//				Name:      roleName,
//				Namespace: objectMeta.GetNamespace(),
//				Annotations: map[string]string{
//					"description": fmt.Sprintf("Role for OIDC Authentication for %s %q", gvk.GroupKind().Kind, objectMeta.Name),
//				},
//			},
//			Rules: []rbacv1.PolicyRule{
//				rbacv1.PolicyRule{
//					APIGroups: []string{""},
//
//					// serviceaccount name
//					ResourceNames: []string{auth.GetOIDCServiceAccountNameForResource(gvk, objectMeta)},
//					Resources:     []string{"serviceaccounts/token"},
//					Verbs:         []string{"create"},
//				},
//			},
//		}
//
//		logger.Errorf("haha: role object created")
//		logger.Errorf("haha: role object %s", role)
//
//		_, err = kubeclient.RbacV1().Roles(objectMeta.Namespace).Create(ctx, role, metav1.CreateOptions{})
//		if err != nil {
//			logger.Errorf("haha: error creating role")
//			logger.Errorf("haha: error %s", err)
//			return fmt.Errorf("could not create OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
//		}
//
//		return nil
//	}
//
//	if err != nil {
//		logger.Errorf("haha: error getting role")
//		logger.Errorf("haha: error getting role %s", err)
//		return fmt.Errorf("could not get OIDC service account role %s/%s for %s: %w", objectMeta.Name, objectMeta.Namespace, gvk.Kind, err)
//
//	}
//	return nil
//}
