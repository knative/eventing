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
	"sort"

	rbacv1listers "k8s.io/client-go/listers/rbac/v1"

	apierrs "k8s.io/apimachinery/pkg/api/errors"

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
	corev1listers "k8s.io/client-go/listers/core/v1"

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
	"knative.dev/eventing/pkg/eventingtls"
	"knative.dev/eventing/pkg/reconciler/apiserversource/resources"
	reconcilersource "knative.dev/eventing/pkg/reconciler/source"
)

const (
	// Name of the corev1.Events emitted from the reconciliation process
	apiserversourceDeploymentCreated = "ApiServerSourceDeploymentCreated"
	apiserversourceDeploymentUpdated = "ApiServerSourceDeploymentUpdated"

	component                 = "apiserversource"
	skipPermissionsAnnotation = "features.knative.dev/apiserversource-skip-permissions-check"
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

	serviceAccountLister       clientv1.ServiceAccountLister
	roleLister                 rbacv1listers.RoleLister
	roleBindingLister          rbacv1listers.RoleBindingLister
	trustBundleConfigMapLister corev1listers.ConfigMapLister
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
			// TODO how does this work with deprecated fields
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
		err := r.createOIDCRole(ctx, source)
		if err != nil {
			logging.FromContext(ctx).Errorw("Failed when creating the OIDC Role for ApiServerSource", zap.Error(err))
			return err
		}

		// Create the rolebinding
		err = r.createOIDCRoleBinding(ctx, source)
		if err != nil {
			logging.FromContext(ctx).Errorw("Failed when creating the OIDC RoleBinding for ApiServerSource", zap.Error(err))
			return err
		}
	}

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

	// We don't check if it really exists because in case it does not exist, the value is an empty string
	// which also serves our purposes as by default we will check permissions
	annotations := source.GetAnnotations()
	skipPermissions := annotations[skipPermissionsAnnotation]
	if skipPermissions == "true" {
		// If skip permissions, mark enough permissions directly
		source.Status.MarkSufficientPermissions()
	} else {
		err = r.runAccessCheck(ctx, source, namespaces)
		if err != nil {
			logging.FromContext(ctx).Errorw("Not enough permission", zap.Error(err))
			return err
		}
	}

	if err := r.propagateTrustBundles(ctx, source); err != nil {
		return err
	}

	// An empty selector targets all namespaces.
	allNamespaces := isEmptySelector(source.Spec.NamespaceSelector)
	ra, err := r.createReceiveAdapter(ctx, source, sinkAddr, namespaces, allNamespaces)
	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to create the receive adapter", zap.Error(err))
		return err
	}

	source.Status.PropagateDeploymentAvailability(ra)

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

	annotations := src.GetAnnotations()
	skipPermissions := annotations[skipPermissionsAnnotation]

	featureFlags := feature.FromContext(ctx)

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
		NodeSelector:  featureFlags.NodeSelector(),
		FailFast:      skipPermissions == "true",
	}

	expected, err := resources.MakeReceiveAdapter(&adapterArgs)
	if err != nil {
		return nil, err
	}

	podTemplate, err := eventingtls.AddTrustBundleVolumes(r.trustBundleConfigMapLister, src, &expected.Spec.Template.Spec)
	if err != nil {
		return nil, fmt.Errorf("failed to add trust bundle volumes: %w", err)
	}
	expected.Spec.Template.Spec = *podTemplate

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
	if len(src.Spec.Resources) == 0 {
		src.Status.MarkSufficientPermissions()
		return nil
	}

	// Run the basic service account access check (This is not OIDC service account)
	user := "system:serviceaccount:" + src.Namespace + ":"
	if src.Spec.ServiceAccountName == "" {
		user += "default"
	} else {
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

// createOIDCRole: this function will call resources package to get the role object
// and then pass to kubeclient to make the actual OIDC role
func (r *Reconciler) createOIDCRole(ctx context.Context, source *v1.ApiServerSource) error {
	roleName := resources.GetOIDCTokenRoleName(source.Name)

	expected, err := resources.MakeOIDCRole(source)
	if err != nil {
		return fmt.Errorf("Cannot create OIDC role for ApiServerSource %s/%s: %w", source.GetName(), source.GetNamespace(), err)
	}

	// By querying roleLister to see whether the role exist or not
	role, err := r.roleLister.Roles(source.GetNamespace()).Get(roleName)

	if apierrs.IsNotFound(err) {
		// If the role does not exist, we will call kubeclient to create it
		role = expected
		_, err = r.kubeClientSet.RbacV1().Roles(source.GetNamespace()).Create(ctx, role, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account role %s/%s for %s: %w", source.GetName(), source.GetNamespace(), "ApiServerSource", err)
		}
	} else {
		// If the role does exist, we will check whether an update is needed
		// By comparing the role's rule
		if !equality.Semantic.DeepEqual(role.Rules, expected.Rules) {
			// If the role's rules are not equal, we will update the role
			role.Rules = expected.Rules
			_, err = r.kubeClientSet.RbacV1().Roles(source.GetNamespace()).Update(ctx, role, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("could not update OIDC service account role %s/%s for %s: %w", source.GetName(), source.GetNamespace(), "ApiServerSource", err)
			}
		} else {
			// If the role does exist and no update is needed, we will just return
			return nil
		}
	}

	return nil
}

// createOIDCRoleBinding:  this function will call resources package to get the rolebinding object
// and then pass to kubeclient to make the actual OIDC rolebinding
func (r *Reconciler) createOIDCRoleBinding(ctx context.Context, source *v1.ApiServerSource) error {
	roleBindingName := resources.GetOIDCTokenRoleBindingName(source.Name)

	expected, err := resources.MakeOIDCRoleBinding(source)
	if err != nil {
		return fmt.Errorf("Cannot create OIDC roleBinding for ApiServerSource %s/%s: %w", source.GetName(), source.GetNamespace(), err)
	}

	// By querying roleBindingLister to see whether the roleBinding exist or not
	roleBinding, err := r.roleBindingLister.RoleBindings(source.GetNamespace()).Get(roleBindingName)
	if apierrs.IsNotFound(err) {
		// If the role does not exist, we will call kubeclient to create it
		roleBinding = expected
		_, err = r.kubeClientSet.RbacV1().RoleBindings(source.GetNamespace()).Create(ctx, roleBinding, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("could not create OIDC service account rolebinding %s/%s for %s: %w", source.GetName(), source.GetNamespace(), "apiserversource", err)
		}
	} else {
		// If the role does exist, we will check whether an update is needed
		// By comparing the role's rule
		if !equality.Semantic.DeepEqual(roleBinding.RoleRef, expected.RoleRef) || !equality.Semantic.DeepEqual(roleBinding.Subjects, expected.Subjects) {
			// If the role's rules are not equal, we will update the role
			roleBinding.RoleRef = expected.RoleRef
			roleBinding.Subjects = expected.Subjects
			_, err = r.kubeClientSet.RbacV1().RoleBindings(source.GetNamespace()).Update(ctx, roleBinding, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("could not update OIDC service account rolebinding %s/%s for %s: %w", source.GetName(), source.GetNamespace(), "apiserversource", err)
			}
		} else {
			// If the role does exist and no update is needed, we will just return
			return nil
		}
	}

	return nil
}

func (r *Reconciler) propagateTrustBundles(ctx context.Context, source *v1.ApiServerSource) error {
	gvk := schema.GroupVersionKind{
		Group:   v1.SchemeGroupVersion.Group,
		Version: v1.SchemeGroupVersion.Version,
		Kind:    "ApiServerSource",
	}
	_, err := eventingtls.PropagateTrustBundles(ctx, r.kubeClientSet, r.trustBundleConfigMapLister, gvk, source)
	return err
}
